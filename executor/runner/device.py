from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Protocol

import psutil
import torch


@dataclass(frozen=True, slots=True)
class MemorySnapshot:
    total_bytes: int
    available_bytes: int

    def __post_init__(self) -> None:
        if self.total_bytes <= 0:
            raise ValueError("total_bytes must be positive")
        if not 0 <= self.available_bytes <= self.total_bytes:
            raise ValueError("available_bytes must be between zero and total_bytes")


@dataclass(frozen=True, slots=True)
class KVCachePlan:
    num_blocks: int
    cache_bytes: int


class ExecutionTimer(Protocol):
    def start(self) -> None: ...

    def stop_ms(self) -> float: ...


class CpuExecutionTimer:
    def __init__(self) -> None:
        self._started_at: float | None = None

    def start(self) -> None:
        self._started_at = time.perf_counter()

    def stop_ms(self) -> float:
        if self._started_at is None:
            raise RuntimeError("execution timer has not been started")
        elapsed_ms = (time.perf_counter() - self._started_at) * 1000
        self._started_at = None
        return elapsed_ms


class CudaExecutionTimer:
    def __init__(self, device: torch.device) -> None:
        if device.type != "cuda":
            raise ValueError("CUDA execution timer requires a CUDA device")

        self.device = device
        with torch.cuda.device(device):
            self._start = torch.cuda.Event(enable_timing=True)
            self._end = torch.cuda.Event(enable_timing=True)
        self._started = False

    def start(self) -> None:
        with torch.cuda.device(self.device):
            self._start.record()
        self._started = True

    def stop_ms(self) -> float:
        if not self._started:
            raise RuntimeError("execution timer has not been started")

        with torch.cuda.device(self.device):
            self._end.record()
            self._end.synchronize()
            elapsed_ms = float(self._start.elapsed_time(self._end))
        self._started = False
        return elapsed_ms


def resolve_device(name: str) -> torch.device:
    try:
        device = torch.device(name)
    except (RuntimeError, ValueError) as exc:
        raise ValueError(f"unsupported device: {name}") from exc

    if device.type == "cpu":
        return torch.device("cpu")
    if device.type != "cuda":
        raise ValueError(f"unsupported device: {name}")
    if not torch.cuda.is_available():
        raise ValueError("CUDA is not available")

    index = device.index if device.index is not None else torch.cuda.current_device()
    if not 0 <= index < torch.cuda.device_count():
        raise ValueError(f"CUDA device index is out of range: {index}")
    return torch.device("cuda", index)


def resolve_torch_dtype(name: str, device: torch.device) -> torch.dtype:
    normalized = name.lower()
    if normalized == "auto":
        if device.type == "cpu":
            return torch.float32
        return torch.bfloat16 if _cuda_supports_bf16(device) else torch.float16

    aliases = {
        "float32": torch.float32,
        "fp32": torch.float32,
        "float16": torch.float16,
        "fp16": torch.float16,
        "bfloat16": torch.bfloat16,
        "bf16": torch.bfloat16,
    }
    try:
        dtype = aliases[normalized]
    except KeyError as exc:
        raise ValueError(f"unsupported dtype: {name}") from exc

    if (
        device.type == "cuda"
        and dtype == torch.bfloat16
        and not _cuda_supports_bf16(device)
    ):
        raise ValueError(f"CUDA device does not support bfloat16: {device}")
    return dtype


def memory_snapshot(device: torch.device) -> MemorySnapshot:
    if device.type == "cpu":
        memory = psutil.virtual_memory()
        return MemorySnapshot(
            total_bytes=int(memory.total),
            available_bytes=int(memory.available),
        )
    if device.type != "cuda":
        raise ValueError(f"unsupported device: {device}")

    torch.cuda.synchronize(device)
    available_bytes, total_bytes = torch.cuda.mem_get_info(device)
    return MemorySnapshot(
        total_bytes=int(total_bytes),
        available_bytes=int(available_bytes),
    )


def plan_kv_cache(
    *,
    device: torch.device,
    before_model: MemorySnapshot,
    after_model: MemorySnapshot,
    gpu_memory_utilization: float,
    kv_cache_memory_bytes: int,
    cache_block_bytes: int,
) -> KVCachePlan:
    if not 0 < gpu_memory_utilization <= 1:
        raise ValueError("gpu_memory_utilization must be in (0, 1]")
    if kv_cache_memory_bytes < 0:
        raise ValueError("kv_cache_memory_bytes must not be negative")
    if cache_block_bytes <= 0:
        raise ValueError("cache_block_bytes must be positive")

    if kv_cache_memory_bytes > 0:
        if kv_cache_memory_bytes > after_model.available_bytes:
            raise ValueError(
                "KV cache memory budget exceeds available memory: "
                f"budget={kv_cache_memory_bytes}, "
                f"available={after_model.available_bytes}"
            )
        budget = kv_cache_memory_bytes
    else:
        if device.type != "cuda":
            raise ValueError("automatic KV cache sizing requires a CUDA device")

        model_footprint = max(
            0,
            before_model.available_bytes - after_model.available_bytes,
        )
        executor_limit = int(
            after_model.total_bytes * gpu_memory_utilization
        )
        budget = min(
            executor_limit - model_footprint,
            after_model.available_bytes,
        )

    num_blocks = budget // cache_block_bytes
    if num_blocks < 1:
        raise ValueError(
            "KV cache memory budget cannot hold one block: "
            f"budget={budget}, block_bytes={cache_block_bytes}"
        )

    return KVCachePlan(
        num_blocks=num_blocks,
        cache_bytes=num_blocks * cache_block_bytes,
    )


def create_execution_timer(device: torch.device) -> ExecutionTimer:
    if device.type == "cpu":
        return CpuExecutionTimer()
    if device.type == "cuda":
        return CudaExecutionTimer(device)
    raise ValueError(f"unsupported device: {device}")


def _cuda_supports_bf16(device: torch.device) -> bool:
    with torch.cuda.device(device):
        return torch.cuda.is_bf16_supported()
