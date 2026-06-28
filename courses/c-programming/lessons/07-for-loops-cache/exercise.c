#include <stdio.h>
#include <stdint.h>
#include <string.h>

// 1. TODO: Include the relative ffmpeg_frame.h header file


/**
 * process_frame - Applies a grayscale conversion to a frame buffer.
 * Modeled after FFmpeg's structural memory handling in libavutil:
 * https://github.com/FFmpeg/FFmpeg/blob/master/libavutil/imgutils.c
 */
void process_frame(AVFrame *frame) {
    // 2. TODO: Implement a nested loop using height and width.
    // Use the formula: frame->data[row * frame->stride + col]
    // to access pixels correctly, skipping the padding bytes at the end of each stride.
    
    
    
    // Simulate processing
    // frame->data[index] = frame->data[index] ^ 0xFF;
}

int main(void) {
    // Simulated AVFrame buffer (1920x1080) with a padded stride of 2048
    uint8_t raw_data[2048 * 1080] = {0};
    AVFrame frame = { .data = raw_data, .width = 1920, .height = 1080, .stride = 2048 };

    process_frame(&frame);

    printf("[FFMPEG] Processing success: Row-major stride traversal complete.\n");
    return 0;
}
