Both commands ran in the requested order and exited with code 0.

The context command reported scope `internal/domain/rule`, zones `domain`, `source`, and `rule`, 60 configured rules, and five missing contract anchors:

- `expansion-only-as-structure`
- `exactly-one-constraint`
- `unique-qualified-id`
- `zone-has-a-path`
- `pattern-lists-zones-without-paths`

The second command printed:
```text
native-workflow-smoke
```

Hook feedback consistently stated: “No supported workflow departure in the supplied evidence.” Its final PostToolUse feedback confirmed execution order and the printed output. It noted that the hook executable exceeded the 65,536-byte evidence limit and that the final report was not yet assessed. Feedback was advisory and granted no approval.