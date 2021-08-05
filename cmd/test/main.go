package main

import "mapdownloader/internal/downloader"

func main() {
	mapInfo := downloader.MapInfo{
		Type:     1,
		Language: "zh",
		MinLng:   "116.312885",
		MaxLng:   "116.500168",
		MinLat:   "39.973805",
		MaxLat:   "39.856128",
		MinZ:     12,
		MaxZ:     17,
		DbPath:   "./mapTiles.db",
	}
	dl := downloader.NewDownLoader(mapInfo, 4096, 4096, 512)
	dl.GetTaskInfo()
	dl.Start()
}
