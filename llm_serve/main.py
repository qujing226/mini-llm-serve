import asyncio
import os
import sys

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
        reponse = ExecuteBatchResponse(
            batch_id = request.batch_id,
            executor_id = "mock-python",
        )

        for item in request.items:
            # mock inference latency
            await asyncio.sleep(0.138)

            reponse.results.append(
                ExecuteResult(
                    task_id = item.task_id,
                )
            )

app = ExecuteServiceASGIApplication(ExecuteServiceImpl())

def main():
    print("Hello from llm-serve!")


if __name__ == "__main__":
    main()
