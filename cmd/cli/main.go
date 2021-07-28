package main

import (
	"mapdownloader/internal/downloader"
)

func main() {
	dl := downloader.NewDownLoader(downloader.MapInfo{
		Type:     1,
		MinZ:     13,
		MaxZ:     15,
		DbPath:   "./mapTiles.db",
		MinLng:   "115.962982",
		MaxLng:   "116.821289",
		MinLat:   "40.22034",
		MaxLat:   "39.692037",
		Language: "zh_CN",
	})
	dl.Start()
}
