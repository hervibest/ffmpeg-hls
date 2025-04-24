# 🎥 Video Encoding & Streaming Backend with Go + FFmpeg + CUDA

This project is a backend service for encoding and streaming videos using HLS format, built with **Golang**, **Fiber**, and **FFmpeg**. It supports **multi-resolution output**, **AES-128 HLS encryption**, and optional **GPU acceleration with CUDA** to speed up video processing.

## 🚀 Features

- 🔁 Encode MP4 videos into HLS `.m3u8` + `.ts` segments
- 🔐 AES-128 encrypted HLS with per-resolution key support
- ⚡ GPU-accelerated encoding via FFmpeg + CUDA (e.g., `h264_nvenc`)
- 🧩 Fiber-based REST API for triggering encoding pipeline
- ☁️ Uploads output to object storage (e.g., MinIO, S3-compatible)
- 📺 Serve HLS master and variant playlists via API
- 🧼 Automatic cleanup after upload

## 🧱 Tech Stack

- **Go + Fiber** – High-performance REST API
- **FFmpeg** – Encoding engine
- **CUDA / NVENC** – GPU-accelerated transcoding (optional)
- **MinIO (S3)** – Media output storage
- **HLS (HTTP Live Streaming)** – Streaming format

## 🛠️ Usage

1. 🔧 Configure `.env` for API server and MinIO credentials.
2. 📦 Send a video via API (or place it in the input folder).
3. 🧪 Call the `EncodeAndUpload` endpoint.
4. 🔗 Get back HLS `master.m3u8` and start streaming.

## 🖥️ Requirements

- Go 1.20+
- FFmpeg with `--enable-nvenc` (for GPU boost)
- NVIDIA GPU with CUDA support (optional but recommended)
- MinIO / S3 bucket configured

## 🗂 Sample API Flow

