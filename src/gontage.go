package gontage

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/nfnt/resize"
)

type drawingInfo struct {
	sprites               []image.Image
	hframes               int
	vframes               int
	vertical_frames_count int
	spritesheet           draw.Image
}

func Gontage(sprite_source_folder string, hframes *int, sprite_resize_px int, single_sprites bool) {
	start := time.Now()
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sprites_folder, err := os.ReadDir(filepath.Join(pwd, sprite_source_folder))
	if err != nil {
		log.Fatal(err)
	} else if len(sprites_folder) == 0 {
		fmt.Println("Looks like folder ", sprite_source_folder, "is empty...")
	}

	if len(sprites_folder) < *hframes {
		*hframes = len(sprites_folder)
	}

	if len(sprites_folder) != 0 {
		var chunkSize int
		if runtime.NumCPU() > 12 && runtime.NumCPU()%4 == 0 {
			chunkSize = runtime.NumCPU() / 4
		} else {
			chunkSize = 6
		}

		var chunk_images_waitgroup sync.WaitGroup
		all_decoded_images := make([]image.Image, len(sprites_folder))
		all_decoded_images_names := make([]string, len(sprites_folder))
		for i := 0; i < len(sprites_folder); i += chunkSize {
			start := i
			end := start + chunkSize
			if end > len(sprites_folder) {
				end = len(sprites_folder)
			}

			chunk_images_waitgroup.Add(1)
			go func(start int, end int) {
				// Ideally decodeImages would write into all_decoded_images directly.
				one_chunk_of_decoded_images, decoded_image_names := decodeImages(sprites_folder[start:end], sprite_source_folder, pwd, &chunk_images_waitgroup)
				for j, decoded_image := range one_chunk_of_decoded_images {
					all_decoded_images[start+j] = decoded_image
					all_decoded_images_names[start+j] = decoded_image_names[j]
				}
			}(start, end)
		}
		chunk_images_waitgroup.Wait()

		// TODO: refactor we use similiar code in other places as well...
		if single_sprites && sprite_resize_px != 0 {
			sprite_source_folder_resized_name := fmt.Sprintf("%v_resized_%vpx", sprite_source_folder, sprite_resize_px)
			os.Mkdir(sprite_source_folder_resized_name, 0755)
			encoder := png.Encoder{CompressionLevel: png.BestSpeed}
			for i, decoded_image := range all_decoded_images {
				resized_sprite_name := fmt.Sprintf("/%v", all_decoded_images_names[i])
				f, err := os.Create(sprite_source_folder_resized_name + resized_sprite_name)
				if err != nil {
					panic(err)
				}
				resized_image := resize.Resize(uint(sprite_resize_px), uint(sprite_resize_px), decoded_image, resize.Lanczos3)
				if err = encoder.Encode(f, resized_image); err != nil {
					log.Printf("failed to encode: %v", err)
				}
				fmt.Println(sprite_source_folder_resized_name + resized_sprite_name)
				f.Close()
			}
			fmt.Println(time.Since(start))
		} else {
			spritesheet_width, spritesheet_height, vframes := calcSheetDimensions(*hframes, all_decoded_images)
			spritesheet := image.NewNRGBA(image.Rect(0, 0, spritesheet_width, spritesheet_height))
			draw.Draw(spritesheet, spritesheet.Bounds(), spritesheet, image.Point{}, draw.Src)
			decoded_images_to_draw_chunked := sliceChunk(all_decoded_images, *hframes)
			var make_spritesheet_wg sync.WaitGroup
			for count_vertical_frames, sprite_chunk := range decoded_images_to_draw_chunked {
				drawing := drawingInfo{
					sprites:     sprite_chunk,
					hframes:     *hframes,
					vframes:     int(vframes),
					spritesheet: spritesheet,
				}
				make_spritesheet_wg.Add(1)
				go func(vertical_frames_count int, sprite_chunk []image.Image) {
					drawing.vertical_frames_count = vertical_frames_count
					defer make_spritesheet_wg.Done()
					drawSpritesheet(drawing)
				}(count_vertical_frames, sprite_chunk)
			}
			make_spritesheet_wg.Wait()
			spritesheet_name := fmt.Sprintf("%v_f%v_v%v.png", sprite_source_folder, len(all_decoded_images), vframes)
			f, err := os.Create(spritesheet_name)
			if err != nil {
				panic(err)
			}
			encoder := png.Encoder{CompressionLevel: png.BestSpeed}
			if sprite_resize_px != 0 {
				resized_spritesheet := resize.Resize(uint(*hframes*sprite_resize_px), uint(int(vframes)*sprite_resize_px),
					spritesheet, resize.Lanczos3)
				if err = encoder.Encode(f, resized_spritesheet); err != nil {
					log.Printf("failed to encode: %v", err)
				}
			} else {
				if err = encoder.Encode(f, spritesheet); err != nil {
					log.Printf("failed to encode: %v", err)
				}
			}

			f.Close()
			fmt.Println(spritesheet_name, ": ", time.Since(start))
		}
	}
}

func decodeImages(sprites_folder []fs.DirEntry, targetFolder string, pwd string, wg *sync.WaitGroup) ([]image.Image, []string) {
	defer wg.Done()
	var sprites_array []image.Image
	var sprites_names []string
	for _, sprite := range sprites_folder {
		if reader, err := os.Open(filepath.Join(pwd, targetFolder, sprite.Name())); err == nil {
			m, _, err := image.Decode(reader)
			if err != nil {
				log.Fatal(err)
			}
			sprites_array = append(sprites_array, m)
			sprites_names = append(sprites_names, sprite.Name())
			reader.Close()
		}
	}
	return sprites_array, sprites_names
}

func drawSpritesheet(drawing drawingInfo) {
	for horizontal_frames_count, sprite_image := range drawing.sprites {
		bounds := sprite_image.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		x0, y0 := horizontal_frames_count*width, drawing.vertical_frames_count*height
		x1, y1 := width*drawing.hframes, height*drawing.vframes
		r := image.Rect(x0, y0, x1, y1)
		draw.Draw(drawing.spritesheet, r, sprite_image, image.Point{}, draw.Over)
	}
}

func sliceChunk[T any](slice []T, chunkSize int) [][]T {
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

func calcSheetDimensions(hframes int, all_decoded_images []image.Image) (int, int, float64) {
	vframes := math.Ceil((float64(len(all_decoded_images)) / float64(hframes)))
	var spritesheet_width int
	var spritesheet_height int
	for _, image := range all_decoded_images[:hframes] {
		spritesheet_width += image.Bounds().Dx()
	}
	for _, image := range all_decoded_images[:int(vframes)] {
		spritesheet_height += image.Bounds().Dy()
	}
	return spritesheet_width, spritesheet_height, vframes
}
