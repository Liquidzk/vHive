#ifndef VHIVE_PLANB_SABRE_WRAPPER_H_
#define VHIVE_PLANB_SABRE_WRAPPER_H_

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
  uint64_t offset;
  uint64_t size;
} snapshare_partition;

typedef struct {
  long mmap_dst_mem;
  long get_partition_info;
  long mmap_snapshot;
  long mmap_decompression_buff;
  long decompress;
  long install_pages;
  long mem_restore_total;
} snapshare_metrics;

typedef enum {
  SNAPSHARE_CODEC_IAA_DEFLATE = 0,
  SNAPSHARE_CODEC_SW_DEFLATE = 1,
  SNAPSHARE_CODEC_ZSTD_1 = 2,
  SNAPSHARE_CODEC_ZSTD_3 = 3,
} snapshare_codec;

typedef struct snapshare_sabre_ctx snapshare_sabre_ctx;

snapshare_sabre_ctx *snapshare_sabre_open(const char *snapshot_path,
                                          snapshare_codec codec,
                                          uint8_t max_hardware_jobs,
                                          int static_huffman);
void snapshare_sabre_close(snapshare_sabre_ctx *ctx);
int snapshare_sabre_compress(snapshare_sabre_ctx *ctx, const uint8_t *base,
                             const snapshare_partition *parts, size_t n_parts);
int snapshare_sabre_decompress(snapshare_sabre_ctx *ctx, size_t region_size,
                               uint8_t **out_region);
void snapshare_sabre_free_region(uint8_t *region, size_t region_size);
void snapshare_sabre_metrics(const snapshare_sabre_ctx *ctx,
                             snapshare_metrics *out);

#ifdef __cplusplus
}
#endif

#endif
