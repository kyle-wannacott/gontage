package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	gontage "github.com/leewannacott/gontage/src"
)

type spritesheet struct {
	sprite_height     int
	sprite_width      int
	amount_of_sprites []int
	hframes           int
	sprite_resize_px  int
}
type folderInfo struct {
	sub_folder_path         string
	folder_name             string
	sub_folder_path_gontage string
	sprite_source_folder    string
}
type cliOptions struct {
	useMontage bool
}

func main() {
	start := time.Now()
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	sprite_source_folder := flag.String("f", "", "Folder name that contains sprites.")
	image_path := flag.String("i", "", "Image path for resizing single image (requires -sr flag)")
	hframes := flag.Int("hf", 8, "Horizontal Frames: Amount of horizontal frames you want in your spritesheet: default 8.")
	sprite_resize_px := flag.Int("sr", 0, "Sprite Resize: Resize each sprite to the pixel value provided.")
	single_sprites := flag.Bool("ss", false, "Single Sprites: Output sprites rather than spritesheet use with -sr flag")
	cpu_threads := flag.Int("t", 0, "CPU threads available (default max available)")
	cut_spritesheet := flag.String("x", "", "Example: -x 128x128. Cut spritesheet into size individual sprites. ")
	parent_folder_path := flag.String("mf", "", "Multiple Folders: path should be parent folder containing sub folders that contain folders with sprites/images in them. Refer to test_multi for example structure.")
	useMontage := flag.Bool("montage", false, "Use montage with -mf instead of gontage (if installed)")
	help := flag.Bool("h", false, "Display help")
	flag.Parse()

	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}
	gontage_args := gontage.GontageArgs{
		Sprite_source_folder:    filepath.Clean(*sprite_source_folder),
		Image_path:              filepath.Clean(*image_path),
		Hframes:                 *hframes,
		Sprite_resize_px_resize: *sprite_resize_px,
		Single_sprites:          *single_sprites,
		Cut_spritesheet:         *cut_spritesheet,
		Cpu_threads:             *cpu_threads,
	}
	if *image_path != "" {
		gontage.ResizeSingleImage(gontage_args)
	} else if *sprite_source_folder != "" {
		gontage.Gontage(gontage_args)
	} else {
		var wg sync.WaitGroup
		if parent_folder_path != nil {
			parent_folder, err := os.ReadDir(filepath.Join(pwd, *parent_folder_path))
			if err != nil {
				log.Fatal(err)
			}
			for _, sub_folder := range parent_folder {
				if err != nil {
					fmt.Println(err)
				}
				sub_folder_path_gontage :=
					filepath.Join(*parent_folder_path, sub_folder.Name())
				sub_folder_path := filepath.Join(pwd, *parent_folder_path, sub_folder.Name())
				folder := folderInfo{
					sub_folder_path:         sub_folder_path,
					sub_folder_path_gontage: sub_folder_path_gontage,
					sprite_source_folder:    *sprite_source_folder,
				}
				amount_of_sprites, folder_names, sprite_height, sprite_width :=
					iterate_folder(sub_folder_path)
				spritesheet := spritesheet{
					sprite_height:     sprite_height,
					sprite_width:      sprite_width,
					amount_of_sprites: amount_of_sprites,
					hframes:           *hframes,
					sprite_resize_px:  *sprite_resize_px,
				}
				cli := cliOptions{
					useMontage: *useMontage,
				}

				if len(amount_of_sprites) == len(folder_names) {
					for i, folder_name := range folder_names {
						wg.Add(1)
						go func(i int, folder_name string) {
							defer wg.Done()
							folder.folder_name = folder_name
							call_gontage_or_montage(i, spritesheet, folder, cli, gontage_args)
						}(i, folder_name)
					}
					wg.Wait()
				}
			}
		}
		fmt.Println("Total time: ", time.Since(start))
	}
}

func call_gontage_or_montage(i int, spritesheet spritesheet, folder folderInfo, cli cliOptions, gargs gontage.GontageArgs) {
	spritesheet_width := spritesheet.hframes
	spritesheet_height := math.Ceil(float64(spritesheet.amount_of_sprites[i]/spritesheet_width) + 1)
	background_type := "transparent"
	geometry_size := fmt.Sprintf("%vx%v", spritesheet.sprite_height, spritesheet.sprite_width)
	input_folder_path := filepath.Join((folder.sub_folder_path), folder.folder_name, "/*")
	tile_size := fmt.Sprintf("%vx%v", spritesheet_width, spritesheet_height)
	sprite_name := fmt.Sprintf("%s_f%d_v%v.png", folder.folder_name, spritesheet.amount_of_sprites[i], spritesheet_height)

	if cli.useMontage {
		out, err := exec.Command("montage", input_folder_path, "-geometry", geometry_size, "-tile", tile_size,
			"-background", background_type, sprite_name).CombinedOutput()
		if err != nil {
			fmt.Println("could not run command: ", err)
		}
		fmt.Println(string(out), filepath.Join(folder.sub_folder_path_gontage, folder.folder_name)+"/*", sprite_name)
	} else {
		gontage_args := gontage.GontageArgs{
			Sprite_source_folder:    filepath.Join(folder.sub_folder_path_gontage, folder.folder_name),
			Hframes:                 spritesheet.hframes,
			Sprite_resize_px_resize: spritesheet.sprite_resize_px,
			Single_sprites:          false,
			Cut_spritesheet:         "",
			Cpu_threads:             gargs.Cpu_threads,
		}
		gontage.Gontage(gontage_args)
	}
}

func iterate_folder(file_path_to_walk string) ([]int, []string, int, int) {
	is_first_sprite_in_directory := true
	folder_names := []string{}
	amount_of_sprites := []int{}
	sprite_height := 0
	sprite_width := 0

	is_containing_folder := true
	filepath.Walk(file_path_to_walk, func(path string, info os.FileInfo, err error) error {
		if !is_containing_folder {
			if err != nil {
				log.Fatalf(err.Error())
			}
			if info.IsDir() {
				folder_path, err := os.ReadDir(path)
				if err != nil {
					log.Fatalf(err.Error())
				}
				amount_of_sprites = append(amount_of_sprites, len(folder_path))
				folder_names = append(folder_names, info.Name())
			}
			if !info.IsDir() && is_first_sprite_in_directory {
				if reader, err := os.Open(path); err == nil {
					m, _, err := image.Decode(reader)
					if err != nil {
						log.Fatal(err)
					}
					bounds := m.Bounds()
					w, h := bounds.Dx(), bounds.Dy()
					sprite_height, sprite_width = h, w
					is_first_sprite_in_directory = false
					reader.Close()
				}
			}
		}
		is_containing_folder = false
		return nil
	})
	return amount_of_sprites, folder_names, sprite_height, sprite_width
}
