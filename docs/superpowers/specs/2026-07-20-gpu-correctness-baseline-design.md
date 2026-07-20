# KVTide 阶段 4A：GPU Correctness Baseline 设计

## 目标

将当前只支持 CPU 的 Qwen Transformers Runner 重构为 CPU/CUDA 设备无关实现，并在 NVIDIA GPU 上完成真实的 `Prefill -> Decode -> Prefix Hit -> ReleaseBlocks` 正确性链路。

本阶段只建立 GPU correctness baseline，不发布性能结论，不实现原生 Paged Attention、GPU batching、mixed batch 或 Native ModelRunner。

## 已确认的配置语义

CUDA 模式默认由用户配置 `gpu_memory_utilization`，Runner 在模型加载后读取 CUDA 实际显存并计算 KV Cache block 数。`kv_cache_memory_bytes` 改为可选的专家级精确覆盖项：

- 未设置时，根据 `gpu_memory_utilization` 自动计算。
- 设置时，使用指定字节数，并校验当前设备可以安全容纳。
- 显式字节数不是与自动预算取最小值；它替代自动预算。
- CPU 模式继续允许显式设置主存预算，避免自动占用大量系统内存。

该语义使普通部署只需要配置利用率，同时保留确定性实验所需的精确容量控制。

## 架构选择

采用单一设备无关 Transformers Runner。

- `executor/runner/transformers.py` 保存唯一真实实现。
- `executor/runner/transformers_cpu.py` 变为兼容导入层，重新导出原有公共符号。
- `executor/runner/factory.py` 不再拒绝 CUDA，而是把 Runtime 配置传给统一 Runner。
- CPU 和 CUDA 共用 BatchBuilder、DynamicCacheAdapter、逐 item forward、采样和结果构造逻辑。
- 设备差异限制在设备解析、dtype 解析、内存预算和计时辅助代码中。

未采用两个独立 Runner，因为它会复制并逐渐分叉关键正确性逻辑。未建立完整 DeviceBackend 框架，因为阶段 5 的 Native Runner 不一定复用 Transformers 执行抽象，当前引入会超出 4A 范围。

## 组件边界

### Device-aware Transformers Runner

`QwenTransformersRunner` 接收 Runner 配置和 Runtime 配置，解析一个确定的 `torch.device`。以下对象必须位于同一设备：

- Hugging Face Qwen 模型；
- `input_ids`；
- `position_ids`；
- PagedKVCache 的 key、value 和 valid-slot tensors；
- DynamicCacheAdapter gather/build 产生的 cache tensors。

执行仍按 batch 中的 item 逐个 forward。该限制会在代码注释和文档中保留，防止把 4A 结果误解为正式性能实现。

### Device helpers

设备辅助代码只负责四项职责：

1. 校验 `cpu`、`cuda` 或带 index 的 CUDA 设备字符串；
2. 根据配置和设备解析 torch dtype；
3. 读取 CPU/CUDA 内存并计算可分配的完整 KV blocks；
4. 为 CPU 和 CUDA 提供具有同一调用方式的 execution timer。

辅助模块不负责模型执行、batch 构造或 KV 生命周期，避免演变成阶段 5 之前的通用后端框架。

### PagedKVCache

PagedKVCache 继续接受 `device`，不引入 CUDA 专用分支。所有内部临时 tensor 跟随 cache tensor 的 device。容量统计必须包括 key、value 和 `valid_slots` 的实际 tensor 字节数，使预算值与真实分配一致。

### DynamicCacheAdapter

DynamicCacheAdapter 保持 batch size 1 限制。它不复制 cache 到 CPU；gather 的输出和 write-back 输入保持在 PagedKVCache 所在设备。

## dtype 语义

配置支持 `auto`、`fp32`/`float32`、`fp16`/`float16` 和 `bf16`/`bfloat16`。

`auto` 的解析规则为：

- CPU 使用 FP32；
- CUDA 在硬件支持 BF16 时使用 BF16；
- 其他 CUDA 设备使用 FP16。

用户显式设置 dtype 时尊重配置。显式要求 CUDA BF16 但设备不支持时启动失败，不静默改变精度。CUDA 示例配置和 Modal smoke test 使用 BF16，不以 FP32 作为 GPU 默认值。

## CUDA 显存预算

初始化顺序如下：

1. 校验目标 CUDA 设备可用；
2. 记录模型加载前的 CUDA free/total memory；
3. 将模型以已解析 dtype 加载到目标设备并切换为 eval；
4. 清理未使用的 PyTorch CUDA allocator cache；
5. 同步设备后再次读取 free/total memory；
6. 计算预算和完整 block 数；
7. 分配 PagedKVCache。

自动预算使用：

```text
executor_limit = floor(total_gpu_memory * gpu_memory_utilization)
model_footprint = max(0, free_before_model - free_after_model)
safe_budget = min(
    executor_limit - model_footprint,
    free_after_model,
)
num_blocks = floor(safe_budget / bytes_per_block)
```

`gpu_memory_utilization` 必须位于 `(0, 1]`。如果计算结果小于一个 block，Runner 启动失败。

显式 `kv_cache_memory_bytes` 必须为正数、不得超过当前 free memory，并至少容纳一个完整 block。最终实际分配始终向下对齐到完整 block。RuntimeInfo 报告实际分配值，不报告未对齐的配置值。

每个 block 的预算包含：

- 所有层的 key elements；
- 所有层的 value elements；
- 所有层对应的 bool `valid_slots` elements。

若实际分配触发 CUDA OOM，错误信息补充目标设备、请求预算、block 数及分配前的 free/total memory，同时保留原始异常作为 cause。

CPU 保持基于 `psutil.virtual_memory()` 的内存信息。CPU 模式要求显式 KV 字节预算，且不得超过可用主存。

## RuntimeInfo

CUDA RuntimeInfo 字段定义为：

- `total_memory_bytes`：目标 CUDA 设备总显存；
- `available_memory_bytes`：分配 PagedKVCache 之前的 CUDA 空闲显存；
- `kv_cache_bytes`：成功分配的完整 KV blocks 的全部 tensor 字节数；
- `num_kv_blocks`：实际成功分配的 block 数；
- `dtype`：解析后的具体 dtype，而不是 `auto`。

RuntimeInfo 在 Runner 初始化结束后固定，与现有服务层的读取方式兼容。

## 执行计时

CPU 使用 `time.perf_counter()`。CUDA 使用当前 stream 上的一对 `torch.cuda.Event(enable_timing=True)`：

1. start event 在 item 的 cache gather 之前记录；
2. end event 在 KV write-back 和采样之后记录；
3. 同步 end event；
4. 使用 `start.elapsed_time(end)` 得到毫秒值。

因此 CUDA `execution_ms` 覆盖 cache gather、model forward、KV write-back 和采样产生的 GPU 工作，不会只测到异步 kernel launch 的 CPU 时间。本阶段保留 protobuf 的整数毫秒字段，不修改协议。

## 执行数据流

每个 ExecuteItem 的执行链保持为：

```text
BatchBuilder
  -> invalidate newly allocated physical blocks
  -> gather existing prefix KV from block_table
  -> create input/position tensors on runner device
  -> Hugging Face forward with DynamicCache
  -> copy only newly appended KV into physical slots
  -> greedy sample on device and return scalar token
  -> synchronized execution timing
```

物理 block 仍属于当前 Executor 实例。本阶段不改变 Go block location、runtime epoch 或跨 Executor 语义；这些属于阶段 4B。

## 错误处理

以下问题在 Runner 启动时立即失败：

- 不支持的 device 字符串；
- 请求 CUDA 但 `torch.cuda.is_available()` 为 false；
- CUDA device index 超出可见设备范围；
- 不支持的 dtype；
- 显式 BF16 与设备能力不兼容；
- utilization 越界；
- KV 预算不足一个 block，或显式预算超过可用内存。

CUDA 配置错误绝不静默回退 CPU。execute 和 release 中已有的 block、slot 及 cache shape 校验继续生效。

## 测试策略

### 本地 CPU 回归

现有 tiny Qwen 测试迁移到新的模块路径，并继续验证：

- Prefill 写入 PagedKVCache；
- Decode gather 并扩展已有 cache；
- 新分配 block 在 forward 前失效旧 slot；
- release 后 block 在所有层不可 gather；
- 旧 `transformers_cpu` 模块仍可导入相同 Runner。

### 无 GPU 的设备单元测试

通过 mock CUDA API 验证：

- `auto` dtype 的 BF16/FP16 分支；
- 显式 dtype 校验；
- 自动预算公式和完整 block 对齐；
- 显式字节覆盖；
- 预算不足与超过 free memory 的错误；
- CUDA Event 的 record、synchronize 和 elapsed-time 调用顺序。

PagedKVCache 的 byte accounting 在 CPU tensor 上测试，因为 element size 和 shape 语义与设备无关。

### 可选本机 CUDA 测试

需要真实 CUDA 的轻量测试使用 skip-if-unavailable，不让无 GPU 的本地与 CI 环境失败。

## Modal GPU smoke test

新增 `executor/modal_smoke.py`，使用 Modal App 和显式 GPU 类型运行真实 Qwen3-0.6B。默认 GPU 为 `L4`，允许通过环境变量改为 `A10` 或 `A100-40GB`。正式 benchmark 后续固定 `A100-40GB`，但本 smoke test 不产生性能结论。

Modal image 使用 Python 3.12，并固定安装与 executor 兼容的 torch、transformers、protobuf 和 psutil 版本。KVTide executor 源码通过 `Image.add_local_dir` 挂载。公开模型权重缓存到可自动创建的 Modal Volume，避免重复下载。

远程 smoke 函数执行并断言：

1. CUDA 可用，解析 dtype 为 BF16 或 FP16；
2. 模型和 PagedKVCache 位于同一 CUDA device；
3. 真实 Qwen Prefill 成功并写入 slots；
4. 使用 Prefill 的采样 token 完成 Decode；
5. 第二个请求引用相同 prefix physical block，并以非零 `computed_tokens` 成功执行，证明 Prefix Hit gather 路径有效；
6. ReleaseBlocks 后对应 valid slots 全部失效；
7. RuntimeInfo 的总显存、可用显存、KV bytes 和 block 数自洽；
8. 每个结果的 execution time 非负且没有 error message。

脚本最终输出 JSON 摘要，包括 GPU 名称、dtype、KV block 数和各阶段是否通过。它不输出吞吐、TTFT、TBT 或性能比较。

## 可复现命令

本地验证：

```bash
cd executor
uv run python -m unittest discover -s tests -v
```

Modal L4 smoke：

```bash
cd executor
uv run --extra modal modal run modal_smoke.py
```

固定 A100-40GB 验收：

```bash
cd executor
KVTIDE_MODAL_GPU=A100-40GB uv run --extra modal modal run modal_smoke.py
```

## 非目标

- 不实现多 item 的单次 GPU forward；
- 不移除 DynamicCacheAdapter；
- 不实现原生 Qwen execution path；
- 不实现原生 Paged Attention；
- 不实现 CUDA graph、专用 stream 或 worker queue；
- 不修改 Go scheduler、block location 或 runtime epoch；
- 不实现跨 Executor KV 传输；
- 不运行或发布 CPU inference benchmark。

## 验收标准

阶段 4A 的完成条件是：

- 所有无 GPU 本地测试通过；
- CUDA 配置、显存预算和计时行为具有可重复的单元测试；
- Qwen3-0.6B 在 Modal NVIDIA GPU 上完成真实 `Prefill -> Decode -> Prefix Hit -> ReleaseBlocks`；
- GPU smoke 输出只用于正确性证明，不被描述为性能结果。
