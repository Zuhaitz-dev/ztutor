#ifndef FFMPEG_FRAME_H
#define FFMPEG_FRAME_H

#include <stdint.h>

/*
** Memory layout descriptors modeled after FFmpeg libavutil.
** Core Subsystem: Video Frame Stride and Alignment
** Reference: https://github.com/FFmpeg/FFmpeg/blob/master/libavutil/imgutils.c
*/
typedef struct {
    uint8_t *data;      /* Raw pixel pointer */
    int width;          /* Visible pixels per row */
    int height;         /* Visible rows */
    int stride;         /* Bytes per row (padded for alignment) */
} AVFrame;

#endif
