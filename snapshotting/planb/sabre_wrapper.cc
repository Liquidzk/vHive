//go:build sabre

#include "sabre_wrapper.h"

#include <sys/mman.h>
#include <sys/stat.h>

#include <cstring>
#include <new>
#include <string>
#include <tuple>

#include "memory_restorator.h"
#include "utils.h"

struct snapshare_sabre_ctx {
  acc::MemoryRestorator *restorator;
  std::string snapshot_path;
  acc::MemoryRestorator::Metrics metrics;
};

namespace {

struct CodecSetting {
  qpl_path_t execution_path;
  acc::MemoryRestorator::AdditionalExecutionPath additional;
};

CodecSetting SettingFor(snapshare_codec codec) {
  switch (codec) {
    case SNAPSHARE_CODEC_SW_DEFLATE:
      return {qpl_path_software, acc::MemoryRestorator::kNone};
    case SNAPSHARE_CODEC_ZSTD_1:
      return {qpl_path_software, acc::MemoryRestorator::kZSTD_1};
    case SNAPSHARE_CODEC_ZSTD_3:
      return {qpl_path_software, acc::MemoryRestorator::kZSTD_3};
    case SNAPSHARE_CODEC_IAA_DEFLATE:
    default:
      return {qpl_path_hardware, acc::MemoryRestorator::kNone};
  }
}

}  // namespace

snapshare_sabre_ctx *snapshare_sabre_open(const char *snapshot_path,
                                          snapshare_codec codec,
                                          uint8_t max_hardware_jobs,
                                          int static_huffman) {
  if (snapshot_path == nullptr) return nullptr;
  const CodecSetting setting = SettingFor(codec);
  uint8_t jobs = max_hardware_jobs == 0 ? 1 : max_hardware_jobs;
  if (codec != SNAPSHARE_CODEC_IAA_DEFLATE) jobs = 1;

  acc::MemoryRestorator::MemoryRestoratotConfig cfg = {
      .execution_path = setting.execution_path,
      .additional_execution_path = setting.additional,
      .partition_hanlding_path =
          acc::MemoryRestorator::kHandleAsScatteredPartitions,
      .sigle_partition_handling_path =
          acc::MemoryRestorator::kHandleWithUffdioCopy,
      .scattered_partition_handling_path =
          static_huffman
              ? acc::MemoryRestorator::kDoStaticHuffmanForScatteredPartitions
              : acc::MemoryRestorator::kDoDynamicHuffmanForScatteredPartitions,
      .restored_memory_owner = acc::MemoryRestorator::kUserApplication,
      .max_hardware_jobs = jobs,
      .passthrough = false,
  };

  auto *ctx = new (std::nothrow) snapshare_sabre_ctx();
  if (ctx == nullptr) return nullptr;
  ctx->snapshot_path = snapshot_path;
  ctx->restorator =
      new (std::nothrow) acc::MemoryRestorator(cfg, ctx->snapshot_path);
  if (ctx->restorator == nullptr || ctx->restorator->Init()) {
    delete ctx->restorator;
    delete ctx;
    return nullptr;
  }
  std::memset(&ctx->metrics, 0, sizeof(ctx->metrics));
  return ctx;
}

void snapshare_sabre_close(snapshare_sabre_ctx *ctx) {
  if (ctx == nullptr) return;
  delete ctx->restorator;
  delete ctx;
}

int snapshare_sabre_compress(snapshare_sabre_ctx *ctx, const uint8_t *base,
                             const snapshare_partition *parts, size_t n_parts) {
  if (ctx == nullptr || base == nullptr || parts == nullptr || n_parts == 0)
    return -1;
  acc::MemoryRestorator::MemoryPartitions source;
  source.reserve(n_parts);
  for (size_t i = 0; i < n_parts; ++i) {
    source.push_back(std::make_tuple(
        const_cast<uint8_t *>(base) + parts[i].offset, parts[i].size));
  }
  if (ctx->restorator->MakeSnapshot(source,
                                    reinterpret_cast<uint64_t>(base)))
    return -1;

  const std::string payload = ctx->snapshot_path + ".snapshot";
  const std::string partitions = ctx->snapshot_path + ".partitions";
  if (chmod(payload.c_str(), 0644) || chmod(partitions.c_str(), 0644)) return -1;
  return 0;
}

int snapshare_sabre_decompress(snapshare_sabre_ctx *ctx, size_t region_size,
                               uint8_t **out_region) {
  if (ctx == nullptr || out_region == nullptr || region_size == 0) return -1;
  utils::m_mmap::Memory region = utils::m_mmap::allocate(region_size);
  if (region.get() == nullptr) return -1;
  if (ctx->restorator->RestoreFromSnapshot(region, region_size, nullptr))
    return -1;
  ctx->metrics = ctx->restorator->GetMetrics();
  *out_region = region.release();
  return 0;
}

void snapshare_sabre_free_region(uint8_t *region, size_t region_size) {
  if (region != nullptr) munmap(region, region_size);
}

void snapshare_sabre_metrics(const snapshare_sabre_ctx *ctx,
                             snapshare_metrics *out) {
  if (ctx == nullptr || out == nullptr) return;
  out->mmap_dst_mem = ctx->metrics.mmap_dst_mem;
  out->get_partition_info = ctx->metrics.get_partition_info;
  out->mmap_snapshot = ctx->metrics.mmap_snapshot;
  out->mmap_decompression_buff = ctx->metrics.mmap_decompression_buff;
  out->decompress = ctx->metrics.decompress;
  out->install_pages = ctx->metrics.install_pages;
  out->mem_restore_total = ctx->metrics.mem_restore_total;
}
