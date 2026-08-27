# Getting started

Reading a multi-shard HuggingFace dataset without downloading it.

This walks through opening [mvaccargiu/gitskills](https://huggingface.co/datasets/mvaccargiu/gitskills),
a dataset whose `artifacts` split is stored as 31 parquet files totalling
several gigabytes, looking at it, and then printing ten rows from the middle.
The whole run takes under 30 seconds and transfers a few megabytes.

## Before starting

The parquet functions are built into `slap`, so nothing needs to be loaded.

```
slap -e "(apropos 'parquet-)"
```

If you are using a `slap` that does not link the package, build the plugin with
`make` and pull it in instead:

```lisp
(require 'parquet "/path/to/slip-parquet")
```

`gitskills` is a gated dataset so an access token is needed. `parquet-open`
picks it up from the environment and only ever sends it to `huggingface.co`.

```
export HF_TOKEN=hf_...
```

## The shard list

A HuggingFace split is a list of shard URLs rather than one file. The list is
published by the hub API:

```
curl -H "Authorization: Bearer $HF_TOKEN" \
  https://huggingface.co/api/datasets/mvaccargiu/gitskills/parquet
```

```json
{"artifacts": {"train": ["https://.../artifacts/train/0.parquet",
                         "https://.../artifacts/train/1.parquet", ...]}}
```

The shards are numbered so the list can just be built:

```lisp
(setq base "https://huggingface.co/api/datasets/mvaccargiu/gitskills/parquet/artifacts/train/")
(setq shards nil)
(dotimes (n 31)
  (push (concatenate 'string base (princ-to-string n) ".parquet") shards))
(setq shards (nreverse shards))
```

## Opening the dataset

Pass the list and the shards are presented as a single table. Row counts are
summed, rows are read shard by shard in order, and paging spans the boundaries
between them.

```lisp
(setq reader (parquet-open shards))

(format t "~a rows across ~a shards, ~a columns~%"
        (send reader :row-count)
        (length shards)
        (send reader :column-count))
```

```
3797117 rows across 31 shards, 30 columns
```

Opening reads only the footer of each shard, which is 31 range requests and
about 25 seconds. No column data has been read yet.

## Looking before reading

Thirty columns is more than fits on a screen, so name the interesting ones.
`parquet-explore` reads only the rows it shows.

```lisp
(setq cols '("repo_full_name" "path" "location_class"))

(parquet-explore reader cols :limit 5 :summary t)
```

```
3797117 rows in 31 shards, 30 columns, 31 row groups, showing 1-5

| repo_full_name                     | path                       | location_class |
| ---------------------------------- | -------------------------- | -------------- |
| williamfiset/algorithms            | .gemini/SKILL.md           | other          |
| alirezarezvani/claude-skills       | .gemini/skills/tc/SKILL.md | skills-dir     |
| lvisei/web-developer-resources     | vue/skill.md               | other          |
| PhongFeng/FFramework               | Tools/ReadMe/Skill.md      | other          |
| 0xNyk/council-of-high-intelligence | skills/council/SKILL.md    | skills-dir     |
```

## Ten rows from the middle

`:offset` reads only the row groups holding the rows asked for, so landing in
the middle of a multi-gigabyte dataset does not read the front of it.
`:each-row` walks the rows without building a list, and `:assoc` hands each one
over keyed by column name.

```lisp
(setq mid (- (floor (send reader :row-count) 2) 5))

(send reader :each-row
      (lambda (row)
        (format t "~a~30t~a~%"
                (cdr (assoc "repo_full_name" row))
                (cdr (assoc "path" row))))
      :assoc cols :offset mid :limit 10)
```

```
aarshps/hora-core             .github/skills/hora-play-store/SKILL.md
Serigy/nika                   .github/skills/generate-sandbox-env/SKILL.md
paradox123/shared-ai-docs     skills-repo/vendor/mattpocock/.agents/skills/triage/SKILL.md
Trompetilla/Skills            skills/otherpowers/field-steward/skill.md
LJT-520/openClaw-backup       skills/otherpowers/field-steward/skill.md
markahesketh/dotfiles         home/.agents/skills/triage/SKILL.md
mohokh67/dotfiles             home/.agents/skills/triage/SKILL.md
eli0shin/.dotfiles            home/.agents/skills/triage/SKILL.md
jonathoneco/dotfiles          home/.agents/skills/triage/SKILL.md
ricardoalt1515/.config        agents/skills/triage/SKILL.md
```

Those ten rows came from row 1898553, which lands in shard 7. Naming the three columns
matters here: each shard is one row group of roughly 525,000 rows, so a read
pulls whole column chunks. Three narrow columns are a few megabytes; asking for
`content` as well would pull most of a 200MB shard.

Close the reader when done. Closing also cancels any read still in flight.

```lisp
(send reader :close)
```

## Running it

Put the whole thing in a file and hand it to `slap`:

```
export HF_TOKEN=hf_...
slap getting-started.lisp
```

`slap` drops into the REPL after the script so the reader can be poked at
further. Redirect stdin to run it and exit:

```
slap getting-started.lisp < /dev/null
```

```lisp
;; getting-started.lisp
(setq base "https://huggingface.co/api/datasets/mvaccargiu/gitskills/parquet/artifacts/train/")
(setq shards nil)
(dotimes (n 31)
  (push (concatenate 'string base (princ-to-string n) ".parquet") shards))
(setq shards (nreverse shards))

(setq reader (parquet-open shards))

(format t "~a rows across ~a shards, ~a columns~%"
        (send reader :row-count)
        (length shards)
        (send reader :column-count))

(setq cols '("repo_full_name" "path" "location_class"))
(parquet-explore reader cols :limit 5 :summary t)

(setq mid (- (floor (send reader :row-count) 2) 5))
(format t "~%rows ~a-~a~%" mid (+ mid 9))
(send reader :each-row
      (lambda (row)
        (format t "~a~30t~a~%"
                (cdr (assoc "repo_full_name" row))
                (cdr (assoc "path" row))))
      :assoc cols :offset mid :limit 10)

(send reader :close)
```

## Where to go next

- `(send reader :statistics "location_class")` gives per row group min, max and
  null counts straight from the footer without reading any column data.
- `(send reader :row-group-info 15)` reports the `shard` a row group came from.
- `(send reader :metadata)` holds the HuggingFace feature spec that names the
  dataset columns.
- `parquet-explore` takes `:format :tsv` for pasting into a spreadsheet.

See the [README](README.md) for the full set of options.
