<!--
  Generator schema for commit messages.
  After squash-merge, the PR title becomes the subject on main.
  Keep the subject identical to the PR title.
-->

<subject>
<area options="[rules,patterns,cli,config,output,vocabulary,extensions,agents,docs,ci]" required=true>: <imperative summary maxlength=72 case=lower endpunctuation=false>
</subject>

<body lines=true maxlength=100 linbullets=true case=sentence tone=imperative endpunctuation=false>
- Why this change exists, not a walkthrough of the diff
</body>

<trailers>
<closes required=true>
Fixes #<ISSUE>
</closes>
<co-authored-by required=false />
</trailers>

<rules>
- Subject shape is `area: imperative summary` with a lowercase verb after the colon.
- Area is the product surface, not a Conventional Commit type. Do not use feat, fix, docs, chore, build, ci, or test as the prefix.
- Do not wrap the subject in parentheses.
- The issue number belongs in the `Fixes #N` trailer.
- Imperative verb: add, fix, report, reject, allow. Not added, adds, adding.
- No period at the end of the subject.
- Body is optional. If the issue already has the problem, success criteria, and note draft, one subject line plus `Fixes #N` is enough.
- Do not paste the release note into the commit. Keep that on the issue and PR.
- One logical change per commit on main. Squash the PR so wip and review-fix commits never land.
- Do not include unrelated formatting in the same commit.
</rules>

<examples>
rules: add a depends_on pattern

Fixes #42

cli: reject unknown flags with a usage error

Fixes #18

patterns: share pattern evaluation across rules

Fixes #7
</examples>