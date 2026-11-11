# 色彩回归样本

`cmyk.jpeg` 来自 Go 源码的 `src/image/testdata/video-001.cmyk.jpeg`，BSD 许可见 `LICENSE-GO`。测试运行时附加 Compact ICC Profiles 的 CMYK 配置。

`pq10.avif` 和 `hlg10.avif` 为本项目生成的 64×16、10 位、全范围中性灰渐变。使用 FFmpeg / libaom：

```text
ffmpeg -f lavfi -i "nullsrc=s=64x16,format=yuv444p10le,geq=lum=1023*X/63:cb=512:cr=512" -frames:v 1 -c:v libaom-av1 -cpu-used 8 -still-picture 1 -crf 0 -color_range pc -colorspace bt2020nc -color_trc smpte2084 -color_primaries bt2020 pq10.avif
```

HLG 将 `smpte2084` 改为 `arib-std-b67`。该 FFmpeg 构建写出的 NCLX 原色和传递函数为 unspecified，因此生成后将容器 `nclx` 标记后的两个大端 uint16 分别设为 9（BT.2020）及 16（PQ）或 18（HLG）。灰阶像素代表对应传递函数下的编码信号值。测试检查真实解码结果的位深、单调性、中性灰、透明度及 SDR 导出配置。
