import asyncio

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



class ExecuteServiceImpl(ExecuteService):
    async def execute_batch(self, request, ctx: RequestContext) -> ExecuteBatchResponse:
        response = ExecuteBatchResponse(
            batch_id = request.batch_id,
            executor_id = "mock-python",
        )

        for item in request.items:
            # mock inference latency
            await asyncio.sleep(0.138)

            response.results.append(
                ExecuteResult(
                    task_id = item.task_id,
                    request_id = item.request_id,
                    output_text = "mock output from python executor",
                    finish_reason = core_pb2.FINISH_REASON_STOP,
                    input_tokens = max(1, len(item.prompt) // 4),
                    output_tokens=max(1, item.max_tokens or 16),
                    execution_ms=138,
                    error_message=""   ,               
                )
            )

        return response

app = ExecuteServiceASGIApplication(ExecuteServiceImpl())

# def main():
    # print("Hello from llm-serve!")


# if __name__ == "__main__":
    # main()
