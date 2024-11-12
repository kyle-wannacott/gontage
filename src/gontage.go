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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dblezek/tga"
	"github.com/nfnt/resize"
)

type drawingInfo struct {
	sprites               []image.Image
	hframes               int
	vframes               int
	vertical_frames_count int
	spritesheet           draw.Image
}

type GontageArgs struct {
	Sprite_source_folder    string
	Hframes                 int
	Sprite_resize_px_resize int
	Single_sprites          bool
	Cut_spritesheet         string
	Convert_sprites         string
	Cpu_threads             int
}

func Gontage(gargs GontageArgs) {
	// sprite_source_folder string, hframes *int, sprite_resize_px_resize int, single_sprites bool, cut_spritesheet bool
	start := time.Now()
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(filepath.Join(pwd, gargs.Sprite_source_folder))
	sprites_folder, err := os.ReadDir(filepath.Join(pwd, gargs.Sprite_source_folder))
	if err != nil {
		log.Fatal(err)
	} else if len(sprites_folder) == 0 {
		fmt.Println("Looks like folder ", gargs.Sprite_source_folder, "is empty...")
	}
	sprites_folder = cleanSpritesFolder(sprites_folder)

	if len(sprites_folder) < gargs.Hframes {
		gargs.Hframes = len(sprites_folder)
	}

	if len(sprites_folder) != 0 {
		var chunkSize int
		if gargs.Cpu_threads > 0 {
			chunkSize = gargs.Cpu_threads
			runtime.GOMAXPROCS(gargs.Cpu_threads)
		} else if runtime.NumCPU() > 12 && runtime.NumCPU()%4 == 0 {
			chunkSize = runtime.NumCPU() / 4
		} else {
			chunkSize = runtime.NumCPU()
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
				one_chunk_of_decoded_images, decoded_image_names := decodeImages(sprites_folder[start:end], gargs.Sprite_source_folder, pwd, &chunk_images_waitgroup)
				for j, decoded_image := range one_chunk_of_decoded_images {
					all_decoded_images[start+j] = decoded_image
					all_decoded_images_names[start+j] = decoded_image_names[j]
				}
			}(start, end)
		}
		chunk_images_waitgroup.Wait()

		if gargs.Single_sprites {
			spritesToResizedSprites(gargs, all_decoded_images, all_decoded_images_names, start)
		} else if gargs.Cut_spritesheet != "" {
			cutSpritesheetIntoSprites(gargs, all_decoded_images, all_decoded_images_names, start)
		} else {
			spritesToSpritesheet(gargs, all_decoded_images, all_decoded_images_names, start)
		}
	}
}

func cleanSpritesFolder(sprites_folder []fs.DirEntry) []fs.DirEntry {
	var temp_sprites_folder []fs.DirEntry
	for _, sprite := range sprites_folder {
		switch filepath.Ext(sprite.Name()) {
		case ".meta":
			continue
		default:
			temp_sprites_folder = append(temp_sprites_folder, sprite)
		}
	}
	sprites_folder = temp_sprites_folder
	return sprites_folder
}

func decodeImages(sprites_folder []fs.DirEntry, targetFolder string, pwd string, wg *sync.WaitGroup) ([]image.Image, []string) {
	defer wg.Done()
	var sprites_array []image.Image
	var sprites_names []string
	for _, sprite := range sprites_folder {
		if !sprite.IsDir() {
			if reader, err := os.Open(filepath.Join(pwd, targetFolder, sprite.Name())); err == nil {
				switch filepath.Ext(sprite.Name()) {
				// TODO: refactor cases with duplicated logic
				case ".tga":
					s, err := tga.Decode(reader)
					if err != nil {
						log.Fatalln(err)
					}
					sprites_array = append(sprites_array, s)
					sprites_names = append(sprites_names, sprite.Name())
					reader.Close()
				default:
					s, t, err := image.Decode(reader)
					if err != nil {
						log.Fatalln(err, t)
					}
					sprites_array = append(sprites_array, s)
					sprites_names = append(sprites_names, sprite.Name())
					reader.Close()
				}
			}
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

func spritesToResizedSprites(gargs GontageArgs, all_decoded_images []image.Image, all_decoded_images_names []string, start time.Time) {
	sprite_source_folder_resized_name := fmt.Sprintf("%v_resized_%vpx", gargs.Sprite_source_folder, gargs.Sprite_resize_px_resize)
	os.Mkdir(sprite_source_folder_resized_name, 0755)
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	for i, decoded_image := range all_decoded_images {
		sprite_name := strings.Split(all_decoded_images_names[i], ".")
		resized_sprite_name := fmt.Sprintf("/%v.png", sprite_name[0])
		f, err := os.Create(sprite_source_folder_resized_name + resized_sprite_name)
		if err != nil {
			panic(err)
		}
		resized_image := resize.Resize(uint(gargs.Sprite_resize_px_resize), uint(gargs.Sprite_resize_px_resize), decoded_image, resize.Lanczos3)
		if err = encoder.Encode(f, resized_image); err != nil {
			log.Printf("failed to encode: %v", err)
		}
		fmt.Println(sprite_source_folder_resized_name + resized_sprite_name)
		f.Close()
	}
	fmt.Println(time.Since(start))
}

func cutSpritesheetIntoSprites(gargs GontageArgs, all_decoded_images []image.Image, all_decoded_images_names []string, start time.Time) {
	image_size := strings.Split(gargs.Cut_spritesheet, "x")
	image_size_x, err := strconv.Atoi(image_size[0])
	image_size_y, err := strconv.Atoi(image_size[1])
	if err != nil {
		log.Fatalln(err)
	}
	var cut_spritesheet_wg sync.WaitGroup
	for i, decoded_image := range all_decoded_images {
		if decoded_image == nil {
			continue
		}
		cut_spritesheet_wg.Add(1)
		var hframes = decoded_image.Bounds().Dx() / image_size_x
		var vframes = decoded_image.Bounds().Dy() / image_size_y
		frame_count := 0
		go func() {
			defer cut_spritesheet_wg.Done()
			for v := range vframes {
				for h := range hframes {
					cutted_image := image.NewNRGBA(image.Rect(h*image_size_x, v*image_size_y, (h*image_size_x)+image_size_x, (v*image_size_y)+image_size_y))
					r := image.Rect(h*image_size_x, v*image_size_y, (h*image_size_x)+image_size_x, (v*image_size_y)+image_size_y)
					draw.Draw(cutted_image, r, decoded_image, image.Point{h * image_size_x, v * image_size_y}, draw.Over)
					folder_name := strings.Split(all_decoded_images_names[i], ".")
					cut_sprite_name := filepath.Join(fmt.Sprintf("%v.png", frame_count))
					os.Mkdir(filepath.Join(gargs.Sprite_source_folder, folder_name[0]), 0755)
					sprite_output := filepath.Join(gargs.Sprite_source_folder, folder_name[0], cut_sprite_name)
					f, err := os.Create(sprite_output)
					if err != nil {
						panic(err)
					}
					encoder := png.Encoder{CompressionLevel: png.BestSpeed}
					if err = encoder.Encode(f, cutted_image); err != nil {
						log.Printf("failed to encode: %v", err)
					}
					frame_count += 1
				}
			}
		}()
	}
	cut_spritesheet_wg.Wait()
	fmt.Println(all_decoded_images_names, ": \n total time: ", time.Since(start))
}

func spritesToSpritesheet(gargs GontageArgs, all_decoded_images []image.Image, all_decoded_images_names []string, start time.Time) {
	spritesheet_width, spritesheet_height, vframes := calcSheetDimensions(gargs.Hframes, all_decoded_images)
	spritesheet := image.NewNRGBA(image.Rect(0, 0, spritesheet_width, spritesheet_height))
	draw.Draw(spritesheet, spritesheet.Bounds(), spritesheet, image.Point{}, draw.Src)
	decoded_images_to_draw_chunked := sliceChunk(all_decoded_images, gargs.Hframes)
	var make_spritesheet_wg sync.WaitGroup
	for count_vertical_frames, sprite_chunk := range decoded_images_to_draw_chunked {
		drawing := drawingInfo{
			sprites:     sprite_chunk,
			hframes:     gargs.Hframes,
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
	spritesheet_name := fmt.Sprintf("%v_f%v_v%v.png", gargs.Sprite_source_folder, len(all_decoded_images), vframes)
	f, err := os.Create(spritesheet_name)
	if err != nil {
		panic(err)
	}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if gargs.Sprite_resize_px_resize != 0 {
		resized_spritesheet := resize.Resize(uint(gargs.Hframes*gargs.Sprite_resize_px_resize), uint(int(vframes)*gargs.Sprite_resize_px_resize),
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
