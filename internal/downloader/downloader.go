package downloader

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"mapdownloader/config"
	"mapdownloader/internal/pool"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/robertkrimen/otto"
)

type Tile struct {
	Count       int
	TilesType   int
	TilesCol    string
	TilesRow    string
	TilesLevel  string
	TilesBinary []byte
}

type MapInfo struct {
	Type     int `json:"type"`
	MinZ     int `json:"minZ"`
	MaxZ     int `json:"maxZ"`
	DbPath   string
	MinLng   string `json:"minLng"`
	MaxLng   string `json:"maxLng"`
	MinLat   string `json:"minLat"`
	MaxLat   string `json:"maxLat"`
	Language string `json:"lang"`
}

type DownLoader struct {
	db        *sql.DB
	jobs      []Tile
	jsVM      *otto.Otto
	mapInfo   MapInfo
	provider  map[int]string
	netClient *http.Client

	capPipe   int
	capQueue  int
	maxWorker int

	errTiles   int
	doneTiles  int
	totalTiles int
}

func NewDownLoader(info MapInfo, capPipe, capQueue, maxWorker int) *DownLoader {
	vm := otto.New()
	vm.Run(config.CoordTransformLib)

	client := &http.Client{}
	client.Transport = &http.Transport{
		MaxIdleConnsPerHost: 5000,
	}
	client.Timeout = time.Duration(20) * time.Second

	return &DownLoader{
		jobs:       make([]Tile, 0),
		jsVM:       vm,
		mapInfo:    info,
		netClient:  client,
		totalTiles: 0,
		capPipe:    capPipe,
		capQueue:   capQueue,
		maxWorker:  maxWorker,
	}
}

func (dl *DownLoader) GetTaskInfo() int {
	jobs := make([]Tile, 0)
	if dl.mapInfo.Type == 0 {
		jobs = dl.getTilesList(0)
	} else if dl.mapInfo.Type == 1 {
		jobs = append(jobs, dl.getTilesList(1)...)
		jobs = append(jobs, dl.getTilesList(2)...)
	} else {
		jobs = dl.getTilesList(0)
	}
	dl.jobs = jobs
	dl.totalTiles = len(jobs)
	return dl.totalTiles
}

func (dl *DownLoader) GetDownPercent() (float64, int, int) {
	return float64(dl.doneTiles+dl.errTiles) / float64(dl.totalTiles), dl.doneTiles, dl.errTiles
}

func (dl *DownLoader) Start() bool {
	if dl.totalTiles == 0 {
		return false
	}
	if dl.mapInfo.Language == "zh" {
		dl.provider = config.PROVIDER_CN
	} else {
		dl.provider = config.PROVIDER_EN
	}
	dl.initDB()
	dl.cleanDB()

	tilesPipe := make(chan Tile, dl.capPipe)
	done := make(chan bool)

	pool := pool.NewDispatcher(dl.maxWorker, dl.capQueue)
	pool.Run()

	go dl.saveTiles(tilesPipe, done)
	for _, v := range dl.jobs {
		tile := v
		job := func() {
			var err error
			url := strings.Replace(dl.provider[tile.TilesType], "{x}", tile.TilesRow, 1)
			url = strings.Replace(url, "{y}", tile.TilesCol, 1)
			url = strings.Replace(url, "{z}", tile.TilesLevel, 1)
			if tile.TilesBinary, err = dl.getTileBinary(url); err != nil {
				fmt.Println("download tile err", err)
				dl.errTiles++
			} else {
				tilesPipe <- tile
			}
		}
		pool.JobQueue <- job
	}
	<-done
	dl.setTask()
	dl.db.Exec(config.CreateIndex)
	dl.db.Close()
	return true
}

func (dl *DownLoader) saveTiles(pipe chan Tile, done chan bool) {
	tx, _ := dl.db.Begin()
	stmt, _ := tx.Prepare("INSERT INTO map(zoom_level,tile_column,tile_row,tile_type,tile_data) values(?,?,?,?,?);")
	defer tx.Commit()
	for {
		select {
		case ti := <-pipe:
			if _, err := stmt.Exec(ti.TilesLevel, ti.TilesCol, ti.TilesRow, ti.TilesType, ti.TilesBinary); err != nil {
				fmt.Println("saveTile err", err)
			} else {
				dl.doneTiles++
			}
		default:
			if (dl.doneTiles+dl.errTiles) == dl.totalTiles && dl.doneTiles != 0 {
				done <- true
				return
			}
		}
	}
}

func (dl *DownLoader) initDB() {
	isExist := dl.exists(dl.mapInfo.DbPath)
	dl.db, _ = sql.Open("sqlite3", dl.mapInfo.DbPath+"?_sync=2&_journal=MEMORY")
	if !isExist {
		dl.db.Exec(config.TileTable)
		dl.db.Exec(config.TaskTable)
	}
}

func (dl *DownLoader) cleanDB() {
	var err error
	clean := func(query string) {
		if err != nil {
			fmt.Println("clean err")
			return
		}
		_, err = dl.db.Exec(query)
	}
	clean("DELETE FROM map")
	clean("DELETE FROM task")
	clean("update sqlite_sequence set seq = 0 where name = 'map'")
	clean("delete from sqlite_sequence where name = 'map'")
	clean("delete from sqlite_sequence")
	clean("VACUUM")
}

func (dl *DownLoader) setTask() {
	date := time.Now().Format("2006-01-02 15:04:05")
	stmt, _ := dl.db.Prepare("INSERT INTO task(id,type,count,version,language,date,maxLevel,minLevel) values(?,?,?,?,?,?,?,?);")
	stmt.Exec(1, dl.mapInfo.Type, dl.totalTiles-dl.errTiles, config.VERSION, dl.mapInfo.Language, date, dl.mapInfo.MaxZ, dl.mapInfo.MinZ)
}

func (dl *DownLoader) getTilesList(mapType int) []Tile {
	jobs := make([]Tile, 0)
	for z := dl.mapInfo.MinZ; z <= dl.mapInfo.MaxZ; z++ {
		minX, minY := dl.getTilesCoordinate(dl.mapInfo.MinLng, dl.mapInfo.MinLat, z)
		maxX, maxY := dl.getTilesCoordinate(dl.mapInfo.MaxLng, dl.mapInfo.MaxLat, z)
		for i := minX; i <= maxX; i++ {
			for j := minY; j <= maxY; j++ {
				jobs = append(jobs, Tile{
					TilesRow:   strconv.Itoa(i),
					TilesCol:   strconv.Itoa(j),
					TilesType:  mapType,
					TilesLevel: strconv.Itoa(z),
				})
			}
		}
	}
	return jobs
}

func (dl *DownLoader) getTilesCoordinate(lng, lat string, z int) (x, y int) {
	flng, _ := strconv.ParseFloat(lng, 64)
	flat, _ := strconv.ParseFloat(lat, 64)
	result, _ := dl.jsVM.Call("TileLnglatTransform.TileLnglatTransformGoogle.lnglatToTile", nil, flng, flat, z)
	tileX, _ := result.Object().Get("tileX")
	tileY, _ := result.Object().Get("tileY")
	x, _ = strconv.Atoi(tileX.String())
	y, _ = strconv.Atoi(tileY.String())
	return
}

func (dl *DownLoader) getTileBinary(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	req.Header.Add("Accept-Language", "zh,en-US;q=0.9,en;q=0.8,zh-CN;q=0.7")
	req.Header.Add("Cache-Control", "max-age=0")
	req.Header.Add("Connection", "Keep-Alive")
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.125 Safari/537.36")
	resp, err := dl.netClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	return data, err
}

func (dl *DownLoader) exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return os.IsExist(err)
	} else {
		return true
	}
}
