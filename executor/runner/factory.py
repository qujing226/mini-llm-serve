from setting import ExecutorConfig

from runner import MockRunner, ModelRunner, Runner


def create_runner(cfg: ExecutorConfig) -> ModelRunner:
    if cfg.runner.model_type == "mock":
        return MockRunner()
    if cfg.runner.model_type == "qwen3":
        return Runner(cfg.runner, cfg.runtime)

    # if cfg.model_type == "cuda":
    #     return CUDAModelRunner(cfg)
    raise ValueError(f"unsupported runner model_type: {cfg.runner.model_type}")
