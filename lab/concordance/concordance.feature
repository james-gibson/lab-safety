@lab-safety
@concordance
@z0-physical
@domain-concordance
Feature: Concordance — a third party's claims against our own

  Each record names a subject both sides name, a predicate, and the two
  claims. `agrees` is computed from them and is not a field of a record, so
  it cannot be asserted — only recomputed.

  A record may not exist without our own claim, anchored by `our_claim.basis`
  to a file in this repo; and a third party's claim is stored as one
  normalised token, never their prose. Those two rules are what stop this
  file being a copy of somebody's catalogue.

  Background:
    Given the records under "records/" are the concordance's source of truth

