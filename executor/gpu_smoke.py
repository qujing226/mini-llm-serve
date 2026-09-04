import argparse
import asyncio
import json
from pathlib import Path

import torch
from kvtide.v1 import block_pb2, core_pb2, executor_pb2
from runner.transformers import Runner
from setting import RunnerConfig, RuntimeConfig


DEFAULT_KV_CACHE_MEMORY_BYTES = 512 * 1024 * 1024


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run the KVTide Qwen GPU correctness smoke test."
    )
    parser.add_argument("--model-path", required=True)
    parser.add_argument("--device", default="cuda:0")
    parser.add_argument("--dtype", default="auto")
    parser.add_argument(
        "--kv-cache-memory-bytes",
        type=int,
        default=DEFAULT_KV_CACHE_MEMORY_BYTES,
        help="Explicit KV cache budget; use 0 to test automatic sizing.",
    )
    parser.add_argument("--gpu-memory-utilization", type=float, default=0.9)
    return parser.parse_args()


def execute_item(
    *,
    work_id: str,
    request_id: str,
    phase: core_pb2.WorkPhase,
    token_ids: list[int],
    computed_tokens: int,
    block_table: list[int],
    allocated_blocks: list[int],
    block_size: int,
) -> executor_pb2.ExecuteItem:
    return executor_pb2.ExecuteItem(
        work_id=work_id,
        request_id=request_id,
        phase=phase,
        token_ids=token_ids,
        computed_tokens=computed_tokens,
        num_new_tokens=len(token_ids),
        kv_blocks=block_pb2.KVBlockMetadata(
            block_size=block_size,
            block_table=block_table,
            allocated_blocks=allocated_blocks,
        ),
        sample=True,
    )


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def require_success(
    result: executor_pb2.ExecuteResult,
    *,
    computed_tokens: int,
) -> None:
    require(not result.error_message, f"execution failed: {result.error_message}")
    require(
        result.computed_tokens == computed_tokens,
        "unexpected computed token count: "
        f"got={result.computed_tokens}, want={computed_tokens}",
    )
    require(result.generated_tokens == 1, "execution did not generate one token")
    require(result.execution_ms >= 0, "execution time must not be negative")


async def run_smoke(args: argparse.Namespace) -> dict[str, object]:
    requested_device = torch.device(args.device)
    require(requested_device.type == "cuda", "GPU smoke requires a CUDA device")
    require(torch.cuda.is_available(), "CUDA is not available")

    model_path = Path(args.model_path)
    require(model_path.is_dir(), f"model path does not exist: {model_path}")

    runner = Runner(
        RunnerConfig(
            executor_id="gpu-smoke",
            model_id=model_path.name,
            model_path=str(model_path),
            model_type="qwen3",
            dtype=args.dtype,
        ),
        RuntimeConfig(
            device=args.device,
            tensor_parallel_size=1,
            gpu_memory_utilization=args.gpu_memory_utilization,
            kv_cache_memory_bytes=args.kv_cache_memory_bytes,
        ),
    )

    require(runner.device.type == "cuda", "runner did not resolve to CUDA")
    require(
        next(runner.model.parameters()).device == runner.device,
        "model is not on the runner device",
    )
    require(
        runner.kv_cache.key_cache.device == runner.device
        and runner.kv_cache.value_cache.device == runner.device
        and runner.kv_cache.valid_slots.device == runner.device,
        "KV cache tensors are not on the runner device",
    )
    require(runner.num_kv_blocks >= 3, "GPU smoke requires at least three KV blocks")

    prefix_tokens = list(range(1, runner.block_size + 1))
    prefill = (
        await runner.execute(
            [
                execute_item(
                    work_id="request-a-prefill",
                    request_id="request-a",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                    token_ids=prefix_tokens,
                    computed_tokens=0,
                    block_table=[0],
                    allocated_blocks=[0],
                    block_size=runner.block_size,
                )
            ]
        )
    )[0]
    require_success(prefill, computed_tokens=runner.block_size)
    require(
        bool(runner.kv_cache.valid_slots[:, 0, :].all().item()),
        "prefill did not fill prefix block 0",
    )

    decode = (
        await runner.execute(
            [
                execute_item(
                    work_id="request-a-decode",
                    request_id="request-a",
                    phase=core_pb2.WORK_PHASE_DECODE,
                    token_ids=[prefill.token_id],
                    computed_tokens=runner.block_size,
                    block_table=[0, 1],
                    allocated_blocks=[1],
                    block_size=runner.block_size,
                )
            ]
        )
    )[0]
    require_success(decode, computed_tokens=0)
    require(
        bool(runner.kv_cache.valid_slots[:, 1, 0].all().item()),
        "decode did not write block 1",
    )

    prefix_hit = (
        await runner.execute(
            [
                execute_item(
                    work_id="request-b-prefix-hit",
                    request_id="request-b",
                    phase=core_pb2.WORK_PHASE_PREFILL,
                    token_ids=[prefill.token_id],
                    computed_tokens=runner.block_size,
                    block_table=[0, 2],
                    allocated_blocks=[2],
                    block_size=runner.block_size,
                )
            ]
        )
    )[0]
    require_success(prefix_hit, computed_tokens=1)
    require(
        bool(runner.kv_cache.valid_slots[:, 2, 0].all().item()),
        "prefix-hit execution did not write block 2",
    )
    require(
        prefix_hit.token_id == decode.token_id,
        "shared-prefix execution produced a different token: "
        f"decode={decode.token_id}, prefix_hit={prefix_hit.token_id}",
    )

    await runner.release_blocks([0])
    require(
        not bool(runner.kv_cache.valid_slots[:, 0, :].any().item()),
        "release did not invalidate prefix block 0",
    )

    info = runner.runtime_info
    require(info.num_kv_blocks == runner.num_kv_blocks, "KV block count mismatch")
    require(
        info.kv_cache_bytes == runner.kv_cache.cache_bytes,
        "KV cache byte count mismatch",
    )
    require(
        0 < info.kv_cache_bytes <= info.available_memory_bytes,
        "KV cache size is inconsistent with available memory",
    )

    return {
        "gpu": torch.cuda.get_device_name(runner.device),
        "device": str(runner.device),
        "dtype": info.dtype,
        "total_memory_bytes": info.total_memory_bytes,
        "available_memory_bytes": info.available_memory_bytes,
        "kv_cache_bytes": info.kv_cache_bytes,
        "num_kv_blocks": info.num_kv_blocks,
        "prefill_execution_ms": prefill.execution_ms,
        "decode_execution_ms": decode.execution_ms,
        "prefix_hit_execution_ms": prefix_hit.execution_ms,
        "prefill": True,
        "decode": True,
        "prefix_hit": True,
        "release": True,
    }


def main() -> None:
    args = parse_args()
    summary = asyncio.run(run_smoke(args))
    print(json.dumps(summary, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
