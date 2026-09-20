# Matched-rate balance extension

The original matched-rate run used four orders per profile. This extension adds the two missing permutations per profile, producing six samples per server and profile when combined with the original evidence.

- Added orders: Fiber/Kruda/Actix and Actix/Kruda/Fiber at profiles 4 and 8.
- Each profile then contains all six permutations exactly once: every server occupies each position twice and each pair appears in both directions three times.
- Offered rates, binaries, contracts, wrk2 parser, acceptance gates, and frozen capacity evidence remain unchanged.
- Original matched-rate artifacts are hashed before and after the extension and are not rewritten.
