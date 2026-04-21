import asyncio
import random

from connectrpc.request import RequestContext

from mini_llm_serve.v1 import core_pb2
from mini_llm_serve.v1.execute_pb2 import (
    ExecuteBatchResponse,
    ExecuteResult,
)

from mini_llm_serve.v1.execute_connect import (
    ExecuteService,
    ExecuteServiceASGIApplication,
)

request_state: dict[str, int] = {}

class ExecuteServiceImpl(ExecuteService):
    async def execute_batch(self, request, ctx: RequestContext) -> ExecuteBatchResponse:
        response = ExecuteBatchResponse(
            batch_id = request.batch_id,
            executor_id = "mock-python",
        )

        for item in request.items:
            if item.phase == core_pb2.WORK_PHASE_PREFILL:
                result = await self._execute_prefill(item)
            elif item.phase == core_pb2.WORK_PHASE_DECODE:
                result = await self._execute_decode(item)
            else:
                result = ExecuteResult(
                    work_id=item.work_id,
                    request_id=item.request_id,
                    done=True,
                    output_text="",
                    finish_reason=core_pb2.FINISH_REASON_ERROR,
                    input_tokens=0,
                    output_tokens=0,
                    execution_ms=0,
                    error_message=f"unsupported work phase: {item.phase}."
                )

            response.results.append(result)
        return response
    
    async def _execute_prefill(self, item) -> ExecuteResult:
        latency_ms = random.randint(80,180)
        await asyncio.sleep(latency_ms / 1000)

        prompt_tokens = max(1, len(item.prompt) // 4)
        request_state[item.request_id] = 0

        return ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            output_text = "",
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            input_tokens = prompt_tokens,
            output_tokens=0,
            execution_ms=latency_ms,
            error_message="",
        )

    async def _execute_decode(self, item) -> ExecuteResult:
        latency_ms = random.randint(20,60)
        await asyncio.sleep(latency_ms/1000)

        generated = request_state.get(item.request_id, 0)

        chunk_tokens = item.decode_tokens_planned or 4
        remaining = max(0, item.max_tokens - generated)
        actual_tokens = min(chunk_tokens, remaining)

        done = actual_tokens == 0 or generated + actual_tokens >= item.max_tokens

        if actual_tokens > 0:
            chunk_index = generated // chunk_tokens
            output_text = f" chunk-{chunk_index}"
        else:
            output_text = ""

        request_state[item.request_id] = generated + actual_tokens

        if done:
            request_state.pop(item.request_id, None)

        return ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done = done,
            output_text=output_text,
            finish_reason = core_pb2.FINISH_REASON_STOP if done else core_pb2.FINISH_REASON_UNSPECIFIED,
            input_tokens = 0,
            output_tokens = generated + actual_tokens,
            execution_ms=latency_ms,
            error_message="",
        )


app = ExecuteServiceASGIApplication(ExecuteServiceImpl())

# def main():
    # print("Hello from llm-serve!")


# if __name__ == "__main__":
    # main()
