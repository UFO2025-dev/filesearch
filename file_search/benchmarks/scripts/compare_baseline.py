#!/usr/bin/env python3
"""compare_baseline.py — Detect performance regressions vs baseline.

Usage:
  python3 compare_baseline.py baseline.txt current.txt [--threshold 20]

Fails (exit 1) if any benchmark regresses by more than --threshold percent.
Uses simple line-based parsing — no external deps.
"""
import sys
import re
import argparse


def parse_benchmarks(path):
    """Returns {name: ns_per_op}."""
    results = {}
    bench_re = re.compile(r"^(Benchmark\S+)\s+\d+\s+([\d.]+)\s+ns/op")
    with open(path) as f:
        for line in f:
            m = bench_re.match(line)
            if m:
                name, ns = m.group(1), float(m.group(2))
                results.setdefault(name, []).append(ns)
    # Use median of multiple runs
    return {k: sorted(v)[len(v) // 2] for k, v in results.items()}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("baseline")
    parser.add_argument("current")
    parser.add_argument("--threshold", type=float, default=20.0,
                        help="Regression threshold in percent (default: 20)")
    args = parser.parse_args()

    baseline = parse_benchmarks(args.baseline)
    current  = parse_benchmarks(args.current)

    regressions = []
    improvements = []

    for name, cur_ns in sorted(current.items()):
        if name not in baseline:
            print(f"  NEW  {name}: {cur_ns:.1f} ns/op")
            continue
        base_ns = baseline[name]
        delta_pct = (cur_ns - base_ns) / base_ns * 100
        if delta_pct > args.threshold:
            regressions.append((name, base_ns, cur_ns, delta_pct))
            print(f"  REGR {name}: {base_ns:.1f} -> {cur_ns:.1f} ns/op (+{delta_pct:.1f}%)")
        elif delta_pct < -5:
            improvements.append((name, delta_pct))
            print(f"  IMPR {name}: {base_ns:.1f} -> {cur_ns:.1f} ns/op ({delta_pct:.1f}%)")
        else:
            print(f"  OK   {name}: {cur_ns:.1f} ns/op ({delta_pct:+.1f}%)")

    print(f"\nSummary: {len(regressions)} regressions, {len(improvements)} improvements")
    if regressions:
        print("\nREGRESSIONS DETECTED — failing CI")
        sys.exit(1)


if __name__ == "__main__":
    main()
