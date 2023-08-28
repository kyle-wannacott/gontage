package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

func main() {
	start := time.Now()

	// targetFolder := flag.String("f", "owner/repo", "folder containing sprites")
	// flag.Parse()
	// fmt.Printf(*targetFolder)

	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sprites_folder, err := os.ReadDir(filepath.Join(pwd, "sprites"))
	if err != nil {
		log.Fatal(err)
	}

	var decoded_images_chunked [][]image.Image
	var decoded_sprites []image.Image
	var chunked_sprite_dir_entries [][]fs.DirEntry
	if runtime.NumCPU() > 12 && runtime.NumCPU()%4 == 0 {
		chunked_sprite_dir_entries = chunkSpriteDirEntries(sprites_folder, runtime.NumCPU()/4)
	} else {
		chunked_sprite_dir_entries = chunkSpriteDirEntries(sprites_folder, 6)
	}

	var chunk_images_waitgroup sync.WaitGroup
	for _, chunked_sprites_entry := range chunked_sprite_dir_entries {
		chunk_images_waitgroup.Add(1)
		go func(chunked_sprites_entry []fs.DirEntry) {
			one_chunk_of_decoded_images := decodeImages(chunked_sprites_entry, pwd, &chunk_images_waitgroup)
			decoded_images_chunked = append(decoded_images_chunked, one_chunk_of_decoded_images)
		}(chunked_sprites_entry)
	}
	chunk_images_waitgroup.Wait()
	for _, image := range decoded_images_chunked {
		decoded_sprites = append(decoded_sprites, image...)
	}

	// old way
	// wg.Add(1)
	// go func() {
	// 	sprites1 = decodeImages(sprites_folder[:mid], pwd, &wg)
	// }()
	// wg.Add(1)
	// go func() {
	// 	sprites2 = decodeImages(sprites_folder[mid:], pwd, &wg)
	// }()

	// sprites = append(sprites, sprites1...)
	// sprites = append(sprites, sprites2...)

	hframes := 8
	vframes := 12
	spritesheet_height := 128 * hframes
	spritesheet_width := 128 * vframes
	spritesheet := image.NewRGBA(image.Rect(0, 0, spritesheet_height, spritesheet_width))
	draw.Draw(spritesheet, spritesheet.Bounds(), spritesheet, image.Point{}, draw.Src)

	decoded_sprites_chunked := chunkDecodedSprites(decoded_sprites, hframes)

	var make_spritesheet_wg sync.WaitGroup
	for count_vertical_frames, sprite_chunk := range decoded_sprites_chunked {
		make_spritesheet_wg.Add(1)
		go func(count_vertical_frames int, sprite_chunk []image.Image) {
			defer make_spritesheet_wg.Done()
			paintSpritesheet(sprite_chunk, hframes, vframes, count_vertical_frames, spritesheet)
		}(count_vertical_frames, sprite_chunk)
	}
	make_spritesheet_wg.Wait()
	f, err := os.Create("spritesheet.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err = encoder.Encode(f, spritesheet); err != nil {
		log.Printf("failed to encode: %v", err)
	}
	fmt.Println(time.Since(start))
}

func decodeImages(sprites_folder []fs.DirEntry, pwd string, wg *sync.WaitGroup) []image.Image {
	defer wg.Done()
	var sprites_array []image.Image
	for _, sprite := range sprites_folder {
		if reader, err := os.Open(filepath.Join(pwd, "sprites", sprite.Name())); err == nil {
			m, _, err := image.Decode(reader)
			if err != nil {
				log.Fatal(err)
			}
			sprites_array = append(sprites_array, m)
			reader.Close()
		}
	}
	return sprites_array
}

func paintSpritesheet(sprites []image.Image, hframes int, vframes int, count_vertical_frames int, spritesheet draw.Image) {
	for count_horizontal_frames, sprite_image := range sprites {
		bounds := sprite_image.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		draw.Draw(spritesheet, image.Rect(count_horizontal_frames*height, count_vertical_frames*width, width*hframes, height*vframes), sprite_image, image.Point{}, draw.Over)
	}
}

func chunkDecodedSprites(slice []image.Image, chunkSize int) [][]image.Image {
	var chunks [][]image.Image
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func chunkSpriteDirEntries(slice []fs.DirEntry, chunkSize int) [][]fs.DirEntry {
	var chunks [][]fs.DirEntry
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}
