from pathlib import Path

import matplotlib.pyplot as plt

series = {
    "dynamic_default": {
        "concurrency": [10, 50, 100, 200],
        "throughput": [26.70, 71.05, 86.49, 137.76],
        "latency": [0.374, 0.684, 1.116, 1.361],
        "color": "#1f77b4",
    },
    "dynamic_fastflush": {
        "concurrency": [10, 50, 100, 200],
        "throughput": [18.60, 61.74, 99.71, 139.30],
        "latency": [0.535, 0.798, 0.958, 1.213],
        "color": "#ff7f0e",
    },
}

root = Path(__file__).resolve().parents[1]
output = root / "assets" / "Stage1_Benchmark_Sweep.svg"

plt.style.use("seaborn-v0_8-whitegrid")

fig, ax1 = plt.subplots(figsize=(10, 6))
ax2 = ax1.twinx()

throughput_lines = []
latency_lines = []

for name, data in series.items():
    throughput_line, = ax1.plot(
        data["concurrency"],
        data["throughput"],
        label=f"{name} throughput",
        color=data["color"],
        linewidth=2.5,
        marker="o",
        markersize=7,
    )
    latency_line, = ax2.plot(
        data["concurrency"],
        data["latency"],
        label=f"{name} avg latency",
        color=data["color"],
        linewidth=2.0,
        linestyle="--",
        marker="s",
        markersize=6,
        alpha=0.9,
    )
    throughput_lines.append(throughput_line)
    latency_lines.append(latency_line)

ax1.set_title("Stage 1 Benchmark: Throughput and Latency vs Concurrency", fontsize=15, pad=16)
ax1.set_xlabel("Concurrency", fontsize=12)
ax1.set_ylabel("Throughput (req/s)", fontsize=12)
ax2.set_ylabel("Average Latency (s)", fontsize=12)

ax1.set_xticks([10, 50, 100, 200])
ax1.set_ylim(0, 160)
ax2.set_ylim(0, 1.6)

handles = throughput_lines + latency_lines
labels = [line.get_label() for line in handles]
ax1.legend(handles, labels, loc="upper left", frameon=True)

fig.tight_layout()
fig.savefig(output, format="svg", bbox_inches="tight")
print(f"saved to {output}")

