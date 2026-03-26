import asyncio
import os
import sys

from connectrpc.request import RequestContext

BASE_DIR = os.path.dirname(__file__)
GEN_DIR = os.path.join(BASE_DIR, "gen")
if GEN_DIR not in sys.path:
    sys.path.insert(0, GEN_DIR)

from mini_llm_serve.v1 import core_pb2
from mini_llm_serve.v1.execute_pb2 import (
    ExecuteBatchResponse,
    ExecuteResult,
)

from mini_llm_serve.v1.execute_connect import(
    ExecuteService,
    ExecuteServiceASGIApplication,
)

class ExecutorServiceImpl(ExecutorService):
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

def main():
    print("Hello from llm-serve!")


if __name__ == "__main__":
    main()
