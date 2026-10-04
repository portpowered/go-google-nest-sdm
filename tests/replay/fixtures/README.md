# Replay fixture provenance

All fixtures in `synthetic/` are hand-authored deterministic examples based on
the checked-in contracts. They are not captures or evidence of live provider
behavior. Every credential and resource identifier is synthetic. JSON bodies
match decoded values; OAuth forms match their complete encoded content. Query
values retain repetition and order. The replay transport rejects unexpected,
duplicate, or mismatched calls before returning a response and requires all
paired exchanges to be consumed.
