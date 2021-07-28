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

	"code.cloudfoundry.org/bytefmt"
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
	Type     int
	MinZ     int
	MaxZ     int
	DbPath   string
	MinLng   string
	MaxLng   string
	MinLat   string
	MaxLat   string
	Language string
}

type DownLoader struct {
	db         *sql.DB
	jsVM       *otto.Otto
	mapInfo    MapInfo
	provider   map[int]string
	netClient  *http.Client
	doneTiles  int
	totalTiles int
}

func NewDownLoader(info MapInfo) *DownLoader {
	vm := otto.New()
	vm.Run(config.CoordTransformLib)

	client := &http.Client{}
	client.Transport = &http.Transport{
		MaxIdleConnsPerHost: 5000,
	}
	client.Timeout = time.Duration(20) * time.Second

	return &DownLoader{
		jsVM:      vm,
		mapInfo:   info,
		netClient: client,
	}
}

func (dl *DownLoader) Start() {
	jobs := make([]Tile, 0)
	if dl.mapInfo.Type == 1 {
		jobs = append(jobs, dl.getTilesList(1)...)
		jobs = append(jobs, dl.getTilesList(2)...)
	} else {
		jobs = dl.getTilesList(1)
	}
	dl.totalTiles = len(jobs)
	fmt.Println("任务数: ", dl.totalTiles, "预计大小: ", bytefmt.ByteSize(uint64(dl.totalTiles*1024*15)))
	if dl.mapInfo.Language == "zh_CN" {
		dl.provider = config.PROVIDER_CN
	} else {
		dl.provider = config.PROVIDER_EN
	}
	dl.initDB()
	dl.cleanDB()

	tilesPipe := make(chan Tile, 4096)
	done := make(chan bool)

	pool := pool.NewDispatcher(500, 2048)
	pool.Run()

	go dl.saveTiles(tilesPipe, done)

	for _, v := range jobs {
		job := func() {
			var err error
			url := strings.Replace(dl.provider[v.TilesType], "{x}", v.TilesRow, 1)
			url = strings.Replace(url, "{y}", v.TilesCol, 1)
			url = strings.Replace(url, "{z}", v.TilesLevel, 1)
			if v.TilesBinary, err = dl.getTileBinary(url); err != nil {
				fmt.Println("download tile err", err)
				dl.totalTiles -= 1
			} else {
				tilesPipe <- v
			}
		}
		pool.JobQueue <- job
	}
	<-done
	dl.db.Exec(config.CreateIndex)
	dl.db.Close()
	fmt.Println("下载完毕")
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
				fmt.Println(dl.doneTiles, dl.totalTiles, dl.doneTiles/dl.totalTiles*100, "%")
			}
		default:
			if dl.doneTiles == dl.totalTiles && dl.doneTiles != 0 {
				fmt.Println("download success")
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
	_, err := dl.db.Exec("DELETE FROM map")
	_, err = dl.db.Exec("DELETE FROM task")
	if err != nil {
		fmt.Println("清理发生错误", err)
	}
	_, err = dl.db.Exec("update sqlite_sequence set seq = 0 where name = 'map'")
	_, err = dl.db.Exec("delete from sqlite_sequence where name = 'map'")
	_, err = dl.db.Exec("delete from sqlite_sequence")
	_, err = dl.db.Exec("VACUUM")
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
