#!/usr/bin/env python3
"""radius.py — compute the radius of a honeypot trigger for a cluster shape.

The formulas are derived in RADIUS.md; this file exists so the worked values
there are checked rather than asserted. It uses only integer arithmetic where
the inputs are integers, and it never mixes the lab's three rung schemes: a
radius is a 42i distance, and nothing here buckets it.

  ./radius.py --check          # assert the values RADIUS.md states
  ./radius.py                  # print the worked configurations
  ./radius.py --shape ...      # one shape: see --help

Nothing is measured here. `w` is a measured weight, `S` is a policy, and the
outputs inherit exactly that status (MATH.md).
"""

from itertools import cycle, islice
from math import ceil, comb


# --- the formulas ---------------------------------------------------------

def contacts_to_fire(sensitivity, weights):
    """Contacts until the accumulated distance crosses `sensitivity`.

    `weights` is the repeating sequence of contact classes the caller uses, e.g.
    (16,) for tools/call only, or (32, 16) for a direct call then a tools/call.
    A sensitivity of 0 fires on the first contact (the silent honeypot).
    """
    if sensitivity <= 0:
        return 1
    total = 0
    for k, w in enumerate(cycle(weights), start=1):
        total += w
        if total >= sensitivity:
            return k
    raise AssertionError("unreachable: weights must be positive")


def cheapest(weights):
    """The class that sets the tolerance bound — the cheapest admissible contact."""
    return min(weights) if weights else 0


def cluster_tolerated(sensitivity, weights, hosts=1, addresses=1):
    """Contacts an adversary is tolerated for across a per-host ledger.

    The ledger is per (host, address), so tolerance is multiplicative in both.
    """
    k_star = contacts_to_fire(sensitivity, weights)
    return addresses * hosts * (k_star - 1)


def sensitivity_for_budget(budget, weights, hosts=1, addresses=1):
    """Largest sensitivity whose cluster-wide tolerance is at most `budget`."""
    w_min = cheapest(weights)
    return w_min * (1 + budget // (hosts * addresses))


def catchment(decoys, tools, samples=1):
    """Probability a uniform sampler touches bait at least once, per host."""
    if samples <= 0 or decoys <= 0:
        return 0.0
    miss = 1.0
    for j in range(samples):
        miss *= (tools - decoys - j) / (tools - j)
    return 1.0 - miss


def catchment_density(density, samples):
    """The same, from a bait density: 1 - (1 - rho)^s."""
    return 1.0 - (1.0 - density) ** samples


def blind_probability(alarms, faults, receivers):
    """P(no honest receiver) when `receivers` of `alarms` are chosen uniformly."""
    if receivers > alarms:
        return 0.0
    return comb(faults, receivers) / comb(alarms, receivers)


def faults(alarms):
    """Byzantine tolerance: F = floor((N-1)/3)."""
    return (alarms - 1) // 3


def time_to_observe(poll_seconds, hops=1):
    """Upper bound on the time from contact to a red light."""
    return hops * poll_seconds


def blast_radius(receivers, observers_per_alarm):
    """Nodes a trigger reddens: the host's own light, the alarms, the observers."""
    return 1 + receivers + receivers * observers_per_alarm


# --- the worked configurations from RADIUS.md ----------------------------

SHAPES = [
    {
        "name": "tripwire, default",
        "sensitivity": 48, "weights": (32, 16), "hosts": 1, "addresses": 1,
        "expect_fire_on": 2,
    },
    {
        "name": "tripwire, silent",
        "sensitivity": 0, "weights": (32, 16), "hosts": 1, "addresses": 1,
        "expect_fire_on": 1,
    },
    {
        "name": "adhd test, 16",
        "sensitivity": 16, "weights": (16,), "hosts": 1, "addresses": 1,
        "expect_fire_on": 1,
    },
    {
        "name": "three hosts, one caller",
        "sensitivity": 48, "weights": (16,), "hosts": 3, "addresses": 1,
        "expect_tolerated": 6,
    },
    {
        "name": "proxy surface, stale stamps",
        "sensitivity": 48, "weights": (2,), "hosts": 1, "addresses": 1,
        "expect_tolerated": 23,
    },
]


def check():
    for shape in SHAPES:
        k_star = contacts_to_fire(shape["sensitivity"], shape["weights"])
        tolerated = cluster_tolerated(
            shape["sensitivity"], shape["weights"],
            shape["hosts"], shape["addresses"],
        )
        if "expect_fire_on" in shape:
            assert k_star == shape["expect_fire_on"], (shape["name"], k_star)
        if "expect_tolerated" in shape:
            assert tolerated == shape["expect_tolerated"], (shape["name"], tolerated)

    # §3: the cluster budget inverse
    assert sensitivity_for_budget(6, (16,), hosts=3) == 48
    assert cluster_tolerated(48, (16,), hosts=3) == 6

    # §4: the lab's own haystack, and the density form agreeing with it
    assert abs(catchment(10000, 10010, 1) - 10000 / 10010) < 1e-12
    assert abs(catchment(10000, 10010, 3) - catchment_density(10000 / 10010, 3)) < 2e-3

    # §5: F, and the receiver bound that makes a cluster unblind
    assert faults(3) == 0 and faults(5) == 1 and faults(7) == 2
    assert blind_probability(3, 0, 1) == 0.0
    assert blind_probability(5, 1, 1) == 0.2
    assert blind_probability(5, 1, 2) == 0.0

    # §6: the two runs
    assert blast_radius(1, 1) == 3          # tripwire: host, alarm, observer
    assert blast_radius(1, 0) == 2          # accounts: host + alarm A

    print("check: ok — every value RADIUS.md states holds")


def table():
    header = f"{'configuration':30} {'S':>4} {'k*':>4} {'tolerated':>10} {'notes'}"
    print(header)
    print("-" * len(header))
    for shape in SHAPES:
        k_star = contacts_to_fire(shape["sensitivity"], shape["weights"])
        tolerated = cluster_tolerated(
            shape["sensitivity"], shape["weights"],
            shape["hosts"], shape["addresses"],
        )
        notes = f"w={list(shape['weights'])} H={shape['hosts']} A={shape['addresses']}"
        print(f"{shape['name']:30} {shape['sensitivity']:>4} {k_star:>4} {tolerated:>10} {notes}")

    print()
    print(f"haystack  D=10000 T=10010  P_c(1) = {catchment(10000, 10010, 1):.4f}")
    for n in (3, 5, 7):
        f = faults(n)
        rowa = " ".join(
            f"m={m}:{blind_probability(n, f, m):.2f}" for m in range(1, min(f + 1, n) + 1)
        )
        print(f"alarms N={n} F={f}  P(blind) {rowa}")


def main(argv):
    if "--check" in argv:
        check()
        return 0
    if "--shape" in argv:
        i = argv.index("--shape")
        try:
            s = int(argv[i + 1])
        except (IndexError, ValueError):
            print("usage: radius.py --shape <sensitivity> [weights,comma,separated]")
            return 2
        weights = (16,)
        if len(argv) > i + 2 and not argv[i + 2].startswith("-"):
            weights = tuple(int(x) for x in argv[i + 2].split(","))
        print(f"k*        = {contacts_to_fire(s, weights)}")
        print(f"tolerated = {contacts_to_fire(s, weights) - 1}")
        print(f"cheapest  = {cheapest(weights)}")
        return 0
    table()
    return 0


if __name__ == "__main__":
    import sys

    raise SystemExit(main(sys.argv[1:]))
