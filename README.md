# Gontage v1.5.0

Create spritesheets from multiple folders of sprites/images up to ~48x+ faster than ImageMagicks `montage` command.

## Install
`go install github.com/kyle-wannacott/gontage@latest`

## Features
* Images to Spritesheet: flags(-f or -mf)
* Images to Resized images: flags (-f -ss -sr)
* Single Image Resize: flags (-i -sr)
* Spritesheet cut into images: flags (-f -x)
* Circular/Square Fading: flags (-fade, -fm) - applies to all operations

## Help:
`gontage -h`

![image](https://github.com/LeeWannacott/gontage/assets/49783296/7b5f2721-5ca8-4508-b072-431536d247bb)

## Examples:

### Single Image Resize:
```bash
gontage -i myimage.png -sr 64
```
This will resize `myimage.png` to 64x64 pixels and save it as `myimage_resized_64px.png`

### Single Image Resize with Fading:
```bash
gontage -i myimage.png -sr 64 -fade 30
```
This will resize and apply circular fading with 30% radius fade to transparency

```bash
gontage -i myimage.jpg -sr 64 -fade 30 -fm s
```
This will resize a JPG image, apply square fading, and automatically save as PNG

**Note:** Fading preserves original colors and only modifies the alpha channel for smooth transparency transitions.

**Fade Values:**
- `0` = No fading (sharp edges)
- `25` = Light fading (25% of radius fades to transparent)
- `50` = Medium fading (50% of radius fades to transparent)
- `75` = Heavy fading (75% of radius fades to transparent)
- `100` = Full fading (entire radius fades from center to edge)

**Fade Modes:**
- `-fm c` = Circular fading (default)
- `-fm s` = Square fading

**Important:** JPG images with fading are automatically converted to PNG format to preserve transparency.

### Folder Processing with Fading:
```bash
gontage -f sprites_folder -hf 4 -sr 32 -fade 25 -fm c
```
Creates a spritesheet with circular faded edges on each sprite

### Individual Resized Sprites with Fading:
```bash
gontage -f sprites_folder -ss -sr 64 -fade 40 -fm s
```
Outputs individual resized sprites with square fading applied (JPG files become PNG)

### Spritesheet Creation:
![image](https://github.com/LeeWannacott/gontage/assets/49783296/c0c35076-5a54-4295-bab0-45385a0dd31d)



## Benchmarking:

### Multiple folders containing sprites -mf:

* gontage was up to ~48x faster than montage at creating multiple spritesheets (tested on a 12 core [AMD 5900x](https://www.amd.com/en/product/10461))

![image](https://github.com/LeeWannacott/gontage/assets/49783296/485911aa-661c-4313-97f4-bfb85aca8100)


### Single folder -f:
* gontage was ~3.5x faster in this instance tested on a 12 core [AMD 5900x](https://www.amd.com/en/product/10461)
![image](https://github.com/LeeWannacott/gontage/assets/49783296/2859f3b9-7c62-4edb-8aed-d1ff2435f942)

* ~2.5x faster on a 2 core 4 thread [i5-4210U](https://www.intel.com/content/www/us/en/products/sku/81016/intel-core-i54210u-processor-3m-cache-up-to-2-70-ghz/specifications.html) Skylake CPU.
![image](https://github.com/LeeWannacott/gontage/assets/49783296/f7070214-278e-4c98-a0b3-e7af0455d932)



At around the same level of compression:

![image](https://github.com/LeeWannacott/gontage/assets/49783296/6aed6d6f-e7ce-4ca1-8d22-172a84bc398e)
 vs.
![image](https://github.com/LeeWannacott/gontage/assets/49783296/e6a5932e-34dd-4995-8ee6-b1d731e0d61c)

## Image comparison:

Reference images [33](https://github.com/LeeWannacott/gontage/blob/main/test_sprites/frame0033.png) - [40](https://github.com/LeeWannacott/gontage/blob/main/test_sprites/frame0033.png)  :

[Gontage:](https://github.com/LeeWannacott/gontage/blob/main/test_sprites_f187_v24_gontage.png)
![image](https://github.com/LeeWannacott/gontage/assets/49783296/ea271798-7a04-4111-860b-80b19a23b86f)



[Montage 7:](https://github.com/LeeWannacott/gontage/blob/main/test_sprites_f187_v24_montage_7.png)
![image](https://github.com/LeeWannacott/gontage/assets/49783296/05e65b17-2752-4ebd-949b-0f1636eed765)


## Other Info:
* Using an appImage for ImageMagick adds around 0.8seconds to startup time when running montage...
* Building an equivalent in NodeJS took around ~700ms (800ms with Bun) for one folder 7x slower than Gontage (~90ms) and 2x slower than Montage (~350ms).
