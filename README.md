# gontage

Create spritesheets from a folder of sprites faster than ImageMagicks montage command.

## benchmarks
gontage is ~3.5x faster in this instance tested on a 12 core [AMD 5900x](https://www.amd.com/en/product/10461)
![image](https://github.com/LeeWannacott/gontage/assets/49783296/2859f3b9-7c62-4edb-8aed-d1ff2435f942)

~2.5x faster on a 2 core 4 thread [i5-4210U](https://www.intel.com/content/www/us/en/products/sku/81016/intel-core-i54210u-processor-3m-cache-up-to-2-70-ghz/specifications.html) Skylake CPU.
![image](https://github.com/LeeWannacott/gontage/assets/49783296/f7070214-278e-4c98-a0b3-e7af0455d932)



At around the same level of compression:

![image](https://github.com/LeeWannacott/gontage/assets/49783296/82ed22ae-4154-4041-9b7d-e3ab448f09ce)
 vs.
![image](https://github.com/LeeWannacott/gontage/assets/49783296/e6a5932e-34dd-4995-8ee6-b1d731e0d61c)

## Image comparison:

Reference images [33](https://github.com/LeeWannacott/gontage/blob/main/test_sprites/frame0033.png) - [40](https://github.com/LeeWannacott/gontage/blob/main/test_sprites/frame0033.png)  :

[Gontage:](https://github.com/LeeWannacott/gontage/blob/main/test_sprites_f187_v24_gontage.png)
![image](https://github.com/LeeWannacott/gontage/assets/49783296/99cc91a5-295a-46d1-ab16-32451ad22db8)


[Montage:](https://github.com/LeeWannacott/gontage/blob/main/test_sprites_f187_v24_montage.png)
![image](https://github.com/LeeWannacott/gontage/assets/49783296/f6c911b9-2bc0-455c-bed9-5a70c07696ad)




