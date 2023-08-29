package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

func main() {
	start := time.Now()
	sprite_source_folder := flag.String("f", "sprites", "Folder name that contains sprites.")
	desired_spritesheet_name := flag.String("n", "my_spritesheet.png", "Your desired spritesheet name.")
	hframes := flag.Int("hframes", 8, "Amount of horizontal sprites you want in your spritesheet: default 8.")
	flag.Parse()

	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sprites_folder, err := os.ReadDir(filepath.Join(pwd, *sprite_source_folder))
	if err != nil {
		log.Fatal(err)
	}

	var chunkSize int
	if runtime.NumCPU() > 12 && runtime.NumCPU()%4 == 0 {
		chunkSize = runtime.NumCPU() / 4
	} else {
		chunkSize = 6
	}
	// fmt.Print(chunked_sprite_names[0:1])

	var chunk_images_waitgroup sync.WaitGroup
	all_decoded_images := make([]image.Image, len(sprites_folder))
	for i := 0; i < len(sprites_folder); i += chunkSize {
		start := i
		end := start + chunkSize
		if end > len(sprites_folder) {
			end = len(sprites_folder)
		}

		chunk_images_waitgroup.Add(1)
		go func(start int, end int) {
			// Ideally decodeImages would write into all_decoded_images directly.
			one_chunk_of_decoded_images := decodeImages(sprites_folder[start:end], *sprite_source_folder, pwd, &chunk_images_waitgroup)
			for j, decoded_image := range one_chunk_of_decoded_images {
				all_decoded_images[start+j] = decoded_image
			}
		}(start, end)
	}

	chunk_images_waitgroup.Wait()

	// fmt.Println("check order ", all_decoded_images)
	// for i, test_image := range all_decoded_images {
	// fmt.Println(i, test_image)
	// }

	vframes := math.Ceil(float64(len(sprites_folder) / *hframes) + 1)
	spritesheet_height := 128 * *hframes
	spritesheet_width := 128 * vframes
	spritesheet := image.NewRGBA(image.Rect(0, 0, spritesheet_height, int(spritesheet_width)))
	draw.Draw(spritesheet, spritesheet.Bounds(), spritesheet, image.Point{}, draw.Src)
	decoded_images_to_draw_chunked := sliceChunker(all_decoded_images, *hframes)

	var make_spritesheet_wg sync.WaitGroup
	for count_vertical_frames, sprite_chunk := range decoded_images_to_draw_chunked {
		make_spritesheet_wg.Add(1)
		go func(count_vertical_frames int, sprite_chunk []image.Image) {
			defer make_spritesheet_wg.Done()
			paintSpritesheet(sprite_chunk, *hframes, int(vframes), count_vertical_frames, spritesheet)
		}(count_vertical_frames, sprite_chunk)
	}
	make_spritesheet_wg.Wait()
	f, err := os.Create(*desired_spritesheet_name)
	if err != nil {
		panic(err)
	}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err = encoder.Encode(f, spritesheet); err != nil {
		log.Printf("failed to encode: %v", err)
	}
	f.Close()
	fmt.Println(time.Since(start))
}

func decodeImages(sprites_folder []fs.DirEntry, targetFolder string, pwd string, wg *sync.WaitGroup) []image.Image {
	defer wg.Done()
	var sprites_array []image.Image
	for _, sprite := range sprites_folder {
		if reader, err := os.Open(filepath.Join(pwd, targetFolder, sprite.Name())); err == nil {
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
		// fmt.Println(sprite_image)
		draw.Draw(spritesheet, image.Rect(count_horizontal_frames*height, count_vertical_frames*width, width*hframes, height*vframes), sprite_image, image.Point{}, draw.Over)
	}
}

func sliceChunker[T any](slice []T, chunkSize int) [][]T {
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}
