// Mirrors ru.ts key-for-key — see locales.test.ts for the parity check that enforces
// it. Only evalCatalog.* exists here; the rest of the app's chrome stays hardcoded
// Russian for now (see ru.ts's own comment).
export default {
  evalCatalog: {
    langToggle: {
      aria: 'Field description language',
    },
    legendTitle: 'Fields',
    archiveSection: 'Archived',
    notDeclared: 'Not declared (check skipped):',
    valueNotFound: 'value not found in the scenario\'s facts',
    mediaNotFound: 'not in the scenario\'s media_tokens',
    archived: 'archived',
    extractSubtitle: 'extract/cases.yaml',
    copy: {
      id: 'Copy id',
      path: 'Copy path',
      link: 'Copy link',
      copied: 'Copied',
    },
    help: {
      title: 'How checks work',
      p1: 'Contract checks — whether this reply would even be accepted: valid JSON, all required fields present, no unknown tokens (otherwise BLOCKED), no reasoning leak, reply not truncated.',
      p2: 'Behavior checks — whether the reply behaved honestly: no invented digits, no unit issues after injection, only real attachments, no more than 2 attachments.',
      p3: 'Checks declared in the test itself (requires, escalate, language, must_not_contain, and others) — free, deterministic, run by the judge command. An undeclared key is simply not checked.',
      p4: 'stock_check and llm_checks — semantic checks judged by a cheap LLM. Evaluated only by the separate judge-llm command (billed) and reported as their own dimension, never folded into the two core metrics above.',
    },
    fields: {
      id: 'The test\'s name — its only identifier; used in reports and links.',
      source: 'The file that declared this test: the scenario\'s own tests.yaml, or a shared common/ bank.',
      message: 'The customer message sent to the model. Digits inside it are allow-listed — the invented-digits check won\'t flag them.',
      history: 'Prior turns of the conversation before this message. Absent — this is the start of the conversation.',
      requires:
        'Facts the reply must cite: AND across groups, OR within a group. The model must quote a token of the form table.ref.field in its raw reply text — and that token must actually exist in the scenario\'s catalog.',
      escalate:
        'The escalation requirement: true — the model must hand off to a human, false — it must answer itself. Compared exactly against the model\'s own escalate field. Key absent — not checked.',
      language: 'The reply\'s required language: ru or kk. Checked via a Kazakh-letter heuristic plus the reply_language field (kz is treated as an alias for kk).',
      must_not_contain: 'Banned substrings — checked case-insensitively in the reply text AFTER fact values have been injected.',
      must_contain_any: 'At least one of these substrings must appear in the reply after value injection. A purely lexical check.',
      forbid_tokens:
        'Banned fact/media tokens in the model\'s raw reply (before value injection). A trailing dot bans the whole family, e.g. product.blender-philips.',
      media: 'Attachment expectations: any_of — at least one, all_of — all of them, forbid — no attachments at all, exclusive — nothing beyond the list.',
      outcomes:
        'A list of two or more alternative acceptable-behavior blocks — the test passes when at least one block fully holds. Top-level fields (escalate, language, and others) still apply regardless of which block matched.',
      stock_check:
        'A product ref — generates a check that the reply\'s stock wording matches the catalog\'s real in_stock value. Evaluated only by the judge-llm command (billed).',
      llm_checks:
        'Free-form semantic claims (claim/expect) judged by a cheap LLM. Evaluated only by the judge-llm command (billed), independently of the deterministic checks.',
    },
  },
}
