# The join decision — the pattern at a glance

**Can a participant decide to join from this page alone? That is the test this page has to
pass.**

## The constraint

If a participant cannot see the pattern at once, it cannot agree to join. And an agent decides
per turn, under partial visibility, so most joins happen **without** the whole picture in view.
Both halves therefore have to hold:

1. **there must be an artifact that fits in one view** — this one, kept short on purpose. The
   argument lives elsewhere and is allowed to be long ([`SLA.md`](SLA.md));
2. **and the model must be safe to enter without having read it** — because in practice it will
   be.

The second is the load-bearing one, and it is not a concession. It is the design.

## What is being offered

| | an SLA | Lab-safety(SLA) |
| --- | --- | --- |
| promises | a level, to each caller | treatment, to each member |
| kind of good | **private** — my latency, my credit | **public** — my protection is your honesty |
| enforced by | a contract, a penalty, a body that adjudicates | a measure derived from the record |
| what it does to demand | **rewards the herd** | **pays the contributors** |

The deal, in full:

> **If you behave as specified, you receive treatment as specified, and the measurement
> underlying the treatment is available for verification.**

Nothing else is promised. There is no latency promise to capture, no better route to discover
by probing, and no reason to arrive at the same moment as everyone else.

## What you can check before joining

Six questions. Each is answerable for your own deployment, in public, without taking anyone's
word for it:

1. Can a member **demote itself** — and can its own report ever *raise* its standing?
2. Can a **credential be replayed**, and does the second attempt fail *visibly*?
3. Can a compromised **surface reach the authority** — or does authority live where the
   surface cannot speak to it?
4. Does an **unexpected interaction rotate** a role, or does it retry until something breaks?
5. Is the **verdict** obtainable without a licence, a subscription, or the cluster's permission?
6. Is the **measure** derived from a record you can re-derive, or asserted by the node it
   describes?

## Why joining without reading the rest is safe

Four asymmetries. Together they mean **you cannot gain by misunderstanding, and cannot lose by
not knowing**:

- **a self-report can only demote you** — a claim can never raise your standing, so the honest
  strategy is weakly dominant and nobody has to be believed;
- **the instrument holds no licence** — the verdict does not depend on your comprehension, your
  subscription, or the cluster's permission;
- **a flap is suppressed, not banned** — a mistake costs standing on a penalty that decays, and
  you return as it decays;
- **the terms are exact and reciprocal** — the same input produces the same treatment, so
  nothing needs to be trusted, only to be checkable.

And the one thing not to rely on: **that you will be believed.** You will not be — and the
design does not need you to be.

## What to read next, and why you probably do not need to

| doc | what it is | must it fit in one view? |
| --- | --- | --- |
| `JOIN.md` | this page — the join decision | **yes** |
| [`SLA.md`](SLA.md) | the argument: the exposure table, idempotence, herd immunity, the reward path, the instrument, BGP routing | no — a mechanism doc may be long |
| [`SPLASH.md`](SPLASH.md) | the trust model after connection | no |
| [`../sheets/workspaces/math.md`](../sheets/workspaces/math.md) | the loss model — when a bit, or an RPC, is lost | no |
| [`README.md`](README.md) | the lab index | no |

Read them when you want to **argue** with the model. Joining does not require it — which is the
reason this page exists at all.
