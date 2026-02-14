#!/usr/bin/env python3
import argparse
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt


TOOL_EGAFETCH = "EGAfetch"
TOOL_PYEGA3 = "pyEGA3"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--csv", required=True, help="Path to results.csv")
    ap.add_argument("--out", default="", help="Optional output image path (png/svg/pdf). If omitted, shows window.")
    ap.add_argument("--only-success", action="store_true", help="Include only rows with exit_code == 0")
    args = ap.parse_args()

    df = pd.read_csv(Path(args.csv))

    required_cols = {"timestamp", "run", "tool", "target_id", "elapsed_seconds", "exit_code", "notes"}
    missing = required_cols - set(df.columns)
    if missing:
        raise SystemExit(f"CSV missing columns: {sorted(missing)}")

    # Keep only the tools we care about
    df = df[df["tool"].isin([TOOL_EGAFETCH, TOOL_PYEGA3])].copy()
    if df.empty:
        raise SystemExit(f"No rows found for tools: {TOOL_EGAFETCH}, {TOOL_PYEGA3}")

    if args.only_success:
        df = df[df["exit_code"] == 0].copy()

    df["run"] = pd.to_numeric(df["run"], errors="coerce")
    df["elapsed_seconds"] = pd.to_numeric(df["elapsed_seconds"], errors="coerce")
    df = df.dropna(subset=["run", "elapsed_seconds"])
    df["run"] = df["run"].astype(int)
    df["elapsed_minutes"] = df["elapsed_seconds"] / 60.0

    target_ids = sorted(df["target_id"].unique())

    for target_id in target_ids:
        sub = df[df["target_id"] == target_id].copy()

        # Pivot to compute speedup metrics per run
        pivot = (
            sub.pivot_table(index=["run", "target_id"], columns="tool",
                            values="elapsed_minutes", aggfunc="first")
              .reset_index()
        )

        have_both = all(c in pivot.columns for c in [TOOL_EGAFETCH, TOOL_PYEGA3])

        if have_both:
            # Time reduction (%): positive means EGAfetch took less time
            pivot["time_reduction_pct"] = (
                (pivot[TOOL_PYEGA3] - pivot[TOOL_EGAFETCH]) / pivot[TOOL_PYEGA3]
            ) * 100.0

            # Speedup (x): >1 means EGAfetch is faster
            pivot["speedup_x"] = pivot[TOOL_PYEGA3] / pivot[TOOL_EGAFETCH]

            # Speed increase (%): (speedup - 1)*100
            pivot["speed_increase_pct"] = (pivot["speedup_x"] - 1.0) * 100.0

        # ---- Plot times (minutes) ----
        plt.figure(figsize=(10, 6))
        for tool_name, g in sub.groupby("tool"):
            g = g.sort_values("run")
            plt.plot(g["run"], g["elapsed_minutes"], marker="o", linewidth=2, label=tool_name)

        # Annotate with speedup + speed increase above the slower line
        if have_both:
            for _, r in pivot.iterrows():
                y_top = max(r[TOOL_PYEGA3], r[TOOL_EGAFETCH])
                label = f"{r['speedup_x']:.2f}× ({r['speed_increase_pct']:.0f}%)"
                plt.annotate(
                    label,
                    (int(r["run"]), float(y_top)),
                    textcoords="offset points",
                    xytext=(0, 8),
                    ha="center",
                    fontsize=9,
                )

        plt.title(f"Download benchmark: {target_id}")
        plt.xlabel("Run")
        plt.ylabel("Elapsed time (minutes)")
        plt.grid(True, alpha=0.3)
        plt.legend(title="Tool")

        # Print summary table
        print(f"\n=== {target_id} ===")
        if have_both:
            show = pivot[["run", TOOL_EGAFETCH, TOOL_PYEGA3,
                         "time_reduction_pct", "speedup_x", "speed_increase_pct"]].sort_values("run")
            print(show.to_string(index=False, float_format=lambda x: f"{x:.3f}"))
            print("\nAverages:")
            print(f"  Mean time reduction: {show['time_reduction_pct'].mean():.2f}%")
            print(f"  Mean speedup:        {show['speedup_x'].mean():.2f}×")
            print(f"  Mean speed increase: {show['speed_increase_pct'].mean():.2f}%")
        else:
            print(f"Not enough data to compute speedup (need both {TOOL_EGAFETCH} and {TOOL_PYEGA3}).")

        plt.tight_layout()
        if args.out:
            out_path = Path(args.out)
            if len(target_ids) > 1:
                out_path = out_path.with_name(f"{out_path.stem}_{target_id}{out_path.suffix}")
            plt.savefig(out_path, dpi=200)
            print(f"Saved plot: {out_path}")
            plt.close()
        else:
            plt.show()


if __name__ == "__main__":
    main()
