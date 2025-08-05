package gontage

import (
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/jpeg"
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

	"image/color"

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
	Image_path              string
	Hframes                 int
	Sprite_resize_px_resize int
	Fade_amount             int
	Fade_mode               string
	Single_sprites          bool
	Cut_spritesheet         string
	Convert_sprites         string
	Cpu_threads             int
	Fix_png_checksum        bool
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
				one_chunk_of_decoded_images, decoded_image_names := decodeImages(sprites_folder[start:end], gargs.Sprite_source_folder, pwd, &chunk_images_waitgroup, gargs.Fade_amount, gargs.Fade_mode, gargs.Fix_png_checksum)
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

func decodeImages(sprites_folder []fs.DirEntry, targetFolder string, pwd string, wg *sync.WaitGroup, fadeAmount int, fadeMode string, fixPngChecksum bool) ([]image.Image, []string) {
	defer wg.Done()
	var sprites_array []image.Image
	var sprites_names []string
	for _, sprite := range sprites_folder {
		if !sprite.IsDir() {
			imagePath := filepath.Join(pwd, targetFolder, sprite.Name())
			if reader, err := os.Open(imagePath); err == nil {
				var decoded_sprite image.Image
				switch filepath.Ext(sprite.Name()) {
				// TODO: refactor cases with duplicated logic
				case ".tga":
					s, err := tga.Decode(reader)
					if err != nil {
						log.Fatalln(err)
					}
					decoded_sprite = s
					reader.Close()
				default:
					s, t, err := image.Decode(reader)
					if err != nil {
						reader.Close()
						// Try to fix PNG checksum errors if enabled and file is PNG
						if fixPngChecksum && strings.ToLower(filepath.Ext(sprite.Name())) == ".png" {
							if fixErr := FixPngChecksum(imagePath); fixErr != nil {
								log.Fatalf("Failed to fix PNG checksum for %s: %v (original error: %v)", imagePath, fixErr, err)
							}
							// Try to decode again after fixing
							if reader2, err2 := os.Open(imagePath); err2 == nil {
								s2, _, err3 := image.Decode(reader2)
								if err3 != nil {
									reader2.Close()
									log.Fatalf("Failed to decode %s even after PNG checksum fix: %v", imagePath, err3)
								}
								decoded_sprite = s2
								reader2.Close()
							} else {
								log.Fatalf("Failed to reopen %s after PNG checksum fix: %v", imagePath, err2)
							}
						} else {
							log.Fatalln(err, t)
						}
					} else {
						decoded_sprite = s
						reader.Close()
					}
				}

				// Apply fading if specified
				if fadeAmount > 0 {
					decoded_sprite = applyFading(decoded_sprite, fadeAmount, fadeMode)
				}

				sprites_array = append(sprites_array, decoded_sprite)
				sprites_names = append(sprites_names, sprite.Name())
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
	encoder_png := png.Encoder{CompressionLevel: png.BestSpeed}
	encoder_jpg := jpeg.Options{Quality: 100}
	// jpeg.Decode(r io.Reader)
	for i, decoded_image := range all_decoded_images {
		sprite_name := strings.Split(all_decoded_images_names[i], ".")

		// Apply resize first
		resized_image := resize.Resize(uint(gargs.Sprite_resize_px_resize), uint(gargs.Sprite_resize_px_resize), decoded_image, resize.Lanczos3)

		// Apply fading if specified
		if gargs.Fade_amount > 0 {
			resized_image = applyFading(resized_image, gargs.Fade_amount, gargs.Fade_mode)
		}

		// Determine output format - force PNG for faded JPG images
		output_as_png := gargs.Fade_amount > 0 && (sprite_name[1] == "jpg" || sprite_name[1] == "jpeg" || sprite_name[1] == "jfif" || sprite_name[1] == "pjpeg" || sprite_name[1] == "pjp")

		var resized_sprite_name string
		if output_as_png {
			resized_sprite_name = fmt.Sprintf("/%v.png", sprite_name[0])
		} else {
			switch sprite_name[1] {
			case "jpg", "jpeg", "jfif", "pjpeg", "pjp":
				resized_sprite_name = fmt.Sprintf("/%v.%v", sprite_name[0], sprite_name[1])
			default:
				resized_sprite_name = fmt.Sprintf("/%v.png", sprite_name[0])
			}
		}

		// Create the output file
		f, err := os.Create(sprite_source_folder_resized_name + resized_sprite_name)
		if err != nil {
			panic(err)
		}

		// Encode based on output format
		if output_as_png || sprite_name[1] == "png" {
			if err = encoder_png.Encode(f, resized_image); err != nil {
				log.Printf("failed to encode: %v", err)
			}
		} else {
			jpeg.Encode(f, resized_image, &encoder_jpg)
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

					// Apply fading if specified
					if gargs.Fade_amount > 0 {
						faded := applyFading(cutted_image, gargs.Fade_amount, gargs.Fade_mode)
						// Convert RGBA to NRGBA
						bounds := faded.Bounds()
						nrgbaImg := image.NewNRGBA(bounds)
						draw.Draw(nrgbaImg, bounds, faded, bounds.Min, draw.Src)
						cutted_image = nrgbaImg
					}
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

	// Note: Fading is applied to individual sprites before assembly, not to the spritesheet itself
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

func applyFading(img image.Image, fadeAmount int, fadeMode string) *image.RGBA {
	if fadeAmount <= 0 || fadeAmount > 100 {
		// Convert to RGBA to maintain consistent return type
		bounds := img.Bounds()
		rgbaImg := image.NewRGBA(bounds)
		draw.Draw(rgbaImg, bounds, img, bounds.Min, draw.Src)
		return rgbaImg
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create new RGBA image to handle transparency properly
	fadedImg := image.NewRGBA(bounds)

	// Calculate dimensions for fading
	centerX := float64(width) / 2.0
	centerY := float64(height) / 2.0

	var fadeDistanceX, fadeDistanceY float64

	if fadeMode == "s" {
		// Square fading
		fadeDistanceX = centerX * (float64(fadeAmount) / 100.0)
		fadeDistanceY = centerY * (float64(fadeAmount) / 100.0)
	} else {
		// Circle fading (default)
		maxRadius := math.Min(centerX, centerY)
		fadeRadius := maxRadius * (float64(fadeAmount) / 100.0)
		fadeDistanceX = fadeRadius
		fadeDistanceY = fadeRadius
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Get original pixel - use direct RGBA conversion to avoid color space issues
			origColor := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)

			var alphaMultiplier float64 = 1.0

			if fadeMode == "s" {
				// Square fading
				dx := math.Abs(float64(x) - centerX)
				dy := math.Abs(float64(y) - centerY)

				// Calculate fade progress based on both dimensions
				var fadeProgress float64

				if dx > centerX-fadeDistanceX && dy > centerY-fadeDistanceY {
					// Corner region - use combined distance
					xProgress := (dx - (centerX - fadeDistanceX)) / fadeDistanceX
					yProgress := (dy - (centerY - fadeDistanceY)) / fadeDistanceY
					fadeProgress = math.Max(xProgress, yProgress)
				} else if dx > centerX-fadeDistanceX {
					// X edge region
					fadeProgress = (dx - (centerX - fadeDistanceX)) / fadeDistanceX
				} else if dy > centerY-fadeDistanceY {
					// Y edge region
					fadeProgress = (dy - (centerY - fadeDistanceY)) / fadeDistanceY
				} else {
					// Inside the non-faded area
					fadeProgress = 0
				}

				if fadeProgress >= 1.0 {
					alphaMultiplier = 0.0 // Fully transparent
				} else if fadeProgress <= 0 {
					alphaMultiplier = 1.0 // Fully opaque
				} else {
					alphaMultiplier = 1.0 - fadeProgress
				}
			} else {
				// Circle fading (default)
				dx := float64(x) - centerX
				dy := float64(y) - centerY
				distance := math.Sqrt(dx*dx + dy*dy)
				maxRadius := math.Min(centerX, centerY)

				if distance <= maxRadius-fadeDistanceX {
					// Inside the non-faded area - full opacity
					alphaMultiplier = 1.0
				} else if distance >= maxRadius {
					// Outside the circle - fully transparent
					alphaMultiplier = 0.0
				} else {
					// In the fade zone - gradient
					fadeProgress := (distance - (maxRadius - fadeDistanceX)) / fadeDistanceX
					alphaMultiplier = 1.0 - fadeProgress
				}
			}

			// Apply alpha multiplier to original alpha and adjust RGB values to avoid artifacts
			newAlpha := uint8(float64(origColor.A) * alphaMultiplier)
			newR := uint8(float64(origColor.R) * alphaMultiplier)
			newG := uint8(float64(origColor.G) * alphaMultiplier)
			newB := uint8(float64(origColor.B) * alphaMultiplier)
			fadedImg.Set(x, y, color.RGBA{
				R: newR,
				G: newG,
				B: newB,
				A: newAlpha,
			})
		}
	}

	return fadedImg
}

func ResizeSingleImage(gargs GontageArgs) {
	start := time.Now()

	if gargs.Sprite_resize_px_resize == 0 {
		fmt.Println("Error: -sr flag is required when using -i flag to specify resize dimensions")
		os.Exit(1)
	}

	// Check if file exists
	if _, err := os.Stat(gargs.Image_path); os.IsNotExist(err) {
		fmt.Printf("Error: Image file '%s' does not exist\n", gargs.Image_path)
		os.Exit(1)
	}

	// Open and decode the image
	reader, err := os.Open(gargs.Image_path)
	if err != nil {
		log.Fatalf("Error opening image: %v", err)
	}
	defer reader.Close()

	var decoded_image image.Image
	switch filepath.Ext(gargs.Image_path) {
	case ".tga":
		decoded_image, err = tga.Decode(reader)
		if err != nil {
			log.Fatalf("Error decoding TGA image: %v", err)
		}
	default:
		decoded_image, _, err = image.Decode(reader)
		if err != nil {
			reader.Close()
			// Try to fix PNG checksum errors if enabled and file is PNG
			if gargs.Fix_png_checksum && strings.ToLower(filepath.Ext(gargs.Image_path)) == ".png" {
				if fixErr := FixPngChecksum(gargs.Image_path); fixErr != nil {
					log.Fatalf("Failed to fix PNG checksum for %s: %v (original error: %v)", gargs.Image_path, fixErr, err)
				}
				// Try to decode again after fixing
				if reader2, err2 := os.Open(gargs.Image_path); err2 == nil {
					decoded_image2, _, err3 := image.Decode(reader2)
					if err3 != nil {
						reader2.Close()
						log.Fatalf("Failed to decode %s even after PNG checksum fix: %v", gargs.Image_path, err3)
					}
					decoded_image = decoded_image2
					reader2.Close()
				} else {
					log.Fatalf("Failed to reopen %s after PNG checksum fix: %v", gargs.Image_path, err2)
				}
			} else {
				log.Fatalf("Error decoding image: %v", err)
			}
		}
	}

	// Resize the image
	resized_image := resize.Resize(uint(gargs.Sprite_resize_px_resize), uint(gargs.Sprite_resize_px_resize), decoded_image, resize.Lanczos3)

	// Apply fading if specified
	if gargs.Fade_amount > 0 {
		resized_image = applyFading(resized_image, gargs.Fade_amount, gargs.Fade_mode)
	}

	// Generate output filename
	file_ext := filepath.Ext(gargs.Image_path)
	file_name_without_ext := strings.TrimSuffix(filepath.Base(gargs.Image_path), file_ext)

	var output_filename string
	var should_output_png bool

	// Check if we need to force PNG output (faded JPG images)
	if gargs.Fade_amount > 0 && (strings.ToLower(file_ext) == ".jpg" || strings.ToLower(file_ext) == ".jpeg" || strings.ToLower(file_ext) == ".jfif" || strings.ToLower(file_ext) == ".pjpeg" || strings.ToLower(file_ext) == ".pjp") {
		output_filename = fmt.Sprintf("%s_resized_%dpx.png", file_name_without_ext, gargs.Sprite_resize_px_resize)
		should_output_png = true
	} else {
		output_filename = fmt.Sprintf("%s_resized_%dpx%s", file_name_without_ext, gargs.Sprite_resize_px_resize, file_ext)
		should_output_png = false
	}

	// Create output file
	output_file, err := os.Create(output_filename)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer output_file.Close()

	// Encode and save the resized image
	if should_output_png {
		encoder_png := png.Encoder{CompressionLevel: png.BestSpeed}
		err = encoder_png.Encode(output_file, resized_image)
	} else {
		switch strings.ToLower(file_ext) {
		case ".jpg", ".jpeg", ".jfif", ".pjpeg", ".pjp":
			encoder_jpg := jpeg.Options{Quality: 100}
			err = jpeg.Encode(output_file, resized_image, &encoder_jpg)
		default:
			encoder_png := png.Encoder{CompressionLevel: png.BestSpeed}
			err = encoder_png.Encode(output_file, resized_image)
		}
	}

	if err != nil {
		log.Fatalf("Error encoding resized image: %v", err)
	}

	fmt.Printf("Resized image saved as: %s (took %v)\n", output_filename, time.Since(start))
}

// FixPngChecksum attempts to fix PNG checksum errors by re-encoding the image
func FixPngChecksum(imagePath string) error {
	// Create a backup of the original file first
	backupPath := imagePath + ".backup"
	if err := CopyFile(imagePath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}

	// Try to fix the PNG by reconstructing it
	img, err := tryDecodePngWithFix(imagePath)
	if err != nil {
		// Restore backup and return error
		CopyFile(backupPath, imagePath)
		return fmt.Errorf("failed to decode and fix PNG: %v", err)
	}

	// Re-encode the image as PNG
	outputFile, err := os.Create(imagePath)
	if err != nil {
		// Restore backup if file creation fails
		CopyFile(backupPath, imagePath)
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(outputFile, img); err != nil {
		// Restore backup if re-encoding fails
		CopyFile(backupPath, imagePath)
		return fmt.Errorf("failed to re-encode PNG: %v", err)
	}

	// Remove backup if successful
	os.Remove(backupPath)
	fmt.Printf("Fixed PNG checksum for: %s\n", imagePath)
	return nil
}

// tryDecodePngWithFix attempts multiple strategies to decode a corrupted PNG
func tryDecodePngWithFix(imagePath string) (image.Image, error) {
	// Strategy 1: Try standard decode first
	if img, err := tryStandardDecode(imagePath); err == nil {
		return img, nil
	}

	// Strategy 2: Try to read and fix PNG chunks manually
	if img, err := tryFixPngChunks(imagePath); err == nil {
		return img, nil
	}

	// Strategy 3: Try to decode ignoring CRC errors by recreating PNG
	return tryRecreateValidPng(imagePath)
}

// tryStandardDecode attempts normal PNG decoding
func tryStandardDecode(imagePath string) (image.Image, error) {
	reader, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	return img, err
}

// tryFixPngChunks attempts to read PNG data and fix common CRC issues
func tryFixPngChunks(imagePath string) (image.Image, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the entire file
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, err
	}

	// Check PNG signature
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		return nil, fmt.Errorf("not a valid PNG file")
	}

	// Create a temporary file with potentially fixed data
	tempPath := imagePath + ".temp"
	defer os.Remove(tempPath)

	// Try to create a valid PNG by skipping CRC validation
	if err := createValidPngFromData(data, tempPath); err != nil {
		return nil, err
	}

	// Try to decode the fixed PNG
	reader, err := os.Open(tempPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	return img, err
}

// createValidPngFromData creates a new PNG file from the image data, recalculating CRCs
func createValidPngFromData(data []byte, outputPath string) error {
	if len(data) < 8 {
		return fmt.Errorf("PNG data too short")
	}

	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()

	// Write PNG signature
	output.Write(data[:8])

	// Process chunks
	pos := 8
	for pos < len(data) {
		if pos+8 > len(data) {
			break
		}

		// Read chunk length
		chunkLen := int(data[pos])<<24 | int(data[pos+1])<<16 | int(data[pos+2])<<8 | int(data[pos+3])
		pos += 4

		if pos+4+chunkLen+4 > len(data) {
			break
		}

		// Read chunk type
		chunkType := data[pos : pos+4]
		pos += 4

		// Read chunk data
		chunkData := data[pos : pos+chunkLen]
		pos += chunkLen

		// Skip original CRC
		pos += 4

		// Write chunk with recalculated CRC
		writeChunk(output, chunkType, chunkData)

		// Stop at IEND chunk
		if string(chunkType) == "IEND" {
			break
		}
	}

	return nil
}

// writeChunk writes a PNG chunk with correct CRC
func writeChunk(w *os.File, chunkType []byte, data []byte) {
	// Write length
	length := uint32(len(data))
	w.Write([]byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)})

	// Write type
	w.Write(chunkType)

	// Write data
	w.Write(data)

	// Calculate and write CRC
	crc := crc32.NewIEEE()
	crc.Write(chunkType)
	crc.Write(data)
	crcValue := crc.Sum32()
	w.Write([]byte{byte(crcValue >> 24), byte(crcValue >> 16), byte(crcValue >> 8), byte(crcValue)})
}

// tryRecreateValidPng attempts to extract image data and create a completely new PNG
func tryRecreateValidPng(imagePath string) (image.Image, error) {
	// This is a last resort - try to extract raw image data and recreate PNG
	// For now, we'll return an error as this would require complex PNG parsing
	return nil, fmt.Errorf("unable to fix PNG - corruption too severe")
}

// CopyFile creates a copy of src at dst
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}
