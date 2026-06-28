#!/usr/bin/env bash
set -euo pipefail

DEFAULT_SUPPORTED_EXTENSIONS="pdf,txt,md,markdown,rst,adoc,docx,xlsx,csv,tsv,json,jsonl,yml,yaml,toml,ini,conf,cfg,html,htm,xml,sh,bash,zsh,fish,py,js,ts,tsx,jsx,go,rs,c,h,cpp,cxx,cc,hpp,java,kt,php,rb,pl,lua,sql,cmake"
POWERPOINT_EXTENSIONS="ppt,pptx,pps,ppsx,pot,potx"
KNOWN_TEXT_FILENAMES="AUTHORS,CHANGELOG,CHANGES,CONTRIBUTING,COPYING,Dockerfile,INSTALL,LICENSE,MAINTAINERS,Makefile,NEWS,README"

scan_dir=""
recursive=0
output_file=""
show_excluded=0
supported_extensions_csv="$DEFAULT_SUPPORTED_EXTENSIONS"
api_base="http://localhost:8000"
knowledge_base_id=""
knowledge_base_name="Local Folder Knowledge Base"
tags_json="[]"
upload=0
max_files=0
fail_fast=0

usage() {
  cat <<'EOF'
Usage:
  scripts/ingest_folder.sh --dir <folder> [options]

Scan a folder and output files that are suitable for the basic Knowledge Service
ingest pipeline. By default this script only filters candidate files. With
--upload, it calls the Knowledge Service API; parsing, chunking, embedding, and
Qdrant writes still happen inside the service.

Options:
  -d, --dir <folder>          Folder to scan.
  -r, --recursive             Scan subdirectories recursively.
  -o, --output <file>         Write matched candidate file paths to a file.
      --upload                Upload matched files to Knowledge Service API.
      --api-base <url>        Knowledge Service API base URL.
                              Default: http://localhost:8000
      --kb-id <id>            Target knowledge base ID. Required with --upload.
      --kb-name <name>        Knowledge base name when ensuring it exists.
      --tags <json>           JSON string array passed as document tags.
                              A JSON object is still accepted for local metadata filtering.
      --max-files <number>    Stop after matching this many files. 0 means no limit.
      --fail-fast             Stop on first upload failure.
      --include-ext <list>    Override supported extensions, comma-separated.
                              Default includes common text, document, config, and code suffixes.
      --show-excluded         Print skipped files and reasons to stderr.
  -h, --help                  Show this help message.

Examples:
  scripts/ingest_folder.sh --dir /path/to/docs
  scripts/ingest_folder.sh --dir /path/to/docs --recursive --output /tmp/kb_candidates.txt
  scripts/ingest_folder.sh --dir /path/to/docs --include-ext pdf,txt,md,docx,xlsx
  scripts/ingest_folder.sh --dir /path/to/docs --recursive --upload --kb-id kb_linux --kb-name "Linux Docs"

Supported by the basic text ingest plan:
  pdf, txt, md, markdown, rst, adoc, docx, xlsx, csv, tsv, json, jsonl,
  yml, yaml, toml, ini, conf, cfg, html, htm, xml, shell scripts,
  common programming language source files, and known text filenames
  such as README, LICENSE, Makefile, Dockerfile.

Explicitly excluded:
  ppt, pptx, pps, ppsx, pot, potx

Notes:
  - PowerPoint files are excluded because slide extraction needs a separate
    parser strategy before reliable chunking and vectorization.
  - Images are not included in the base set because they require OCR.
  - With --upload, this script calls the Knowledge Service API. Parsing,
    chunking, embedding, and Qdrant writes still happen in the service.
EOF
}

fail() {
  printf '[error] %s\n' "$1" >&2
  exit 1
}

normalize_ext() {
  local ext="$1"
  ext="${ext#.}"
  printf '%s' "${ext,,}"
}

csv_contains_ext() {
  local needle
  needle="$(normalize_ext "$1")"
  local csv="$2"
  local item

  IFS=',' read -r -a items <<<"$csv"
  for item in "${items[@]}"; do
    item="$(normalize_ext "${item//[[:space:]]/}")"
    if [[ "$item" == "$needle" ]]; then
      return 0
    fi
  done

  return 1
}

csv_contains_value() {
  local needle="$1"
  local csv="$2"
  local item

  IFS=',' read -r -a items <<<"$csv"
  for item in "${items[@]}"; do
    item="${item//[[:space:]]/}"
    if [[ "$item" == "$needle" ]]; then
      return 0
    fi
  done

  return 1
}

json_get() {
  local key="$1"
  python3 -c '
import json
import sys

key = sys.argv[1]
data = json.load(sys.stdin)
value = data
for part in key.split("."):
    value = value.get(part) if isinstance(value, dict) else None
print("" if value is None else value)
' "$key"
}

ensure_kb() {
  local payload
  payload="$(python3 -c '
import json
import sys

print(json.dumps({
    "id": sys.argv[1],
    "name": sys.argv[2],
    "description": "Local folder ingest knowledge base",
    "docType": "GENERAL",
    "chunkStrategy": {
        "type": "SEMANTIC_TEXT",
        "chunkSize": 1600,
        "overlap": 200
    },
    "retrievalStrategy": {
        "mode": "VECTOR",
        "topK": 10,
        "scoreThreshold": 0.0
    }
}, ensure_ascii=False))
' "$knowledge_base_id" "$knowledge_base_name")"

  curl -fsS \
    -X POST "$api_base/api/v1/knowledge-bases" \
    -H "Content-Type: application/json" \
    -d "$payload" >/dev/null
}

health_check() {
  curl -fsS "$api_base/healthz" >/dev/null
}

upload_file() {
  local file="$1"
  curl -fsS \
    -X POST "$api_base/api/v1/knowledge-bases/$knowledge_base_id/documents" \
    -F "file=@${file}" \
    -F "tags=${tags_json}"
}

record_excluded() {
  local reason="$1"
  local file="$2"

  if [[ "$show_excluded" -eq 1 ]]; then
    printf '[skip] %-18s %s\n' "$reason" "$file" >&2
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -d|--dir)
      [[ $# -ge 2 ]] || fail "$1 requires a folder path"
      scan_dir="$2"
      shift 2
      ;;
    -r|--recursive)
      recursive=1
      shift
      ;;
    -o|--output)
      [[ $# -ge 2 ]] || fail "$1 requires an output file path"
      output_file="$2"
      shift 2
      ;;
    --include-ext)
      [[ $# -ge 2 ]] || fail "$1 requires a comma-separated extension list"
      supported_extensions_csv="$2"
      shift 2
      ;;
    --show-excluded)
      show_excluded=1
      shift
      ;;
    --upload)
      upload=1
      shift
      ;;
    --api-base)
      [[ $# -ge 2 ]] || fail "$1 requires an API base URL"
      api_base="${2%/}"
      shift 2
      ;;
    --kb-id)
      [[ $# -ge 2 ]] || fail "$1 requires a knowledge base ID"
      knowledge_base_id="$2"
      shift 2
      ;;
    --kb-name)
      [[ $# -ge 2 ]] || fail "$1 requires a knowledge base name"
      knowledge_base_name="$2"
      shift 2
      ;;
    --tags)
      [[ $# -ge 2 ]] || fail "$1 requires a JSON array or object"
      tags_json="$2"
      shift 2
      ;;
    --max-files)
      [[ $# -ge 2 ]] || fail "$1 requires a number"
      max_files="$2"
      shift 2
      ;;
    --fail-fast)
      fail_fast=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[[ -n "$scan_dir" ]] || fail "missing required --dir <folder>"
[[ -d "$scan_dir" ]] || fail "folder does not exist: $scan_dir"
if [[ "$upload" -eq 1 ]]; then
  [[ -n "$knowledge_base_id" ]] || fail "--kb-id is required with --upload"
  python3 -c 'import json, sys; value=json.loads(sys.argv[1]); assert isinstance(value, (list, dict))' "$tags_json" \
    || fail "--tags must be a JSON array or object"
  health_check || fail "Knowledge Service API is not reachable: $api_base"
  ensure_kb || fail "failed to create or update knowledge base: $knowledge_base_id"
  printf '[knowledge-base] id=%s name=%s\n' "$knowledge_base_id" "$knowledge_base_name" >&2
fi

if [[ -n "$output_file" ]]; then
  mkdir -p "$(dirname "$output_file")"
  : >"$output_file"
fi

total_count=0
matched_count=0
excluded_count=0
uploaded_count=0
failed_count=0

find_args=("$scan_dir")
if [[ "$recursive" -eq 0 ]]; then
  find_args+=("-maxdepth" "1")
fi
find_args+=("-type" "f" "-print0")

while IFS= read -r -d '' file; do
  total_count=$((total_count + 1))
  basename="$(basename "$file")"

  if [[ "$basename" == "~$"* ]]; then
    excluded_count=$((excluded_count + 1))
    record_excluded "office_temp" "$file"
    continue
  fi

  if [[ "$basename" != *.* ]]; then
    if ! csv_contains_value "$basename" "$KNOWN_TEXT_FILENAMES"; then
      excluded_count=$((excluded_count + 1))
      record_excluded "no_extension" "$file"
      continue
    fi
    ext="known-text"
  else
    ext="$(normalize_ext "${basename##*.}")"
  fi

  if [[ "$ext" != "known-text" ]] && csv_contains_ext "$ext" "$POWERPOINT_EXTENSIONS"; then
    excluded_count=$((excluded_count + 1))
    record_excluded "powerpoint" "$file"
    continue
  fi

  if [[ "$ext" != "known-text" ]] && ! csv_contains_ext "$ext" "$supported_extensions_csv"; then
    excluded_count=$((excluded_count + 1))
    record_excluded "unsupported .$ext" "$file"
    continue
  fi

  matched_count=$((matched_count + 1))
  if [[ -n "$output_file" ]]; then
    printf '%s\n' "$file" >>"$output_file"
  else
    printf '%s\n' "$file"
  fi

  if [[ "$upload" -eq 1 ]]; then
    printf '[upload] %s\n' "$file" >&2
    if response="$(upload_file "$file")"; then
      uploaded_count=$((uploaded_count + 1))
      document_id="$(printf '%s' "$response" | json_get data.id)"
      job_id="$(printf '%s' "$response" | json_get data.jobId)"
      chunk_count="$(printf '%s' "$response" | json_get data.chunkCount)"
      printf '[ready] documentId=%s jobId=%s chunks=%s\n' "$document_id" "$job_id" "$chunk_count" >&2
    else
      failed_count=$((failed_count + 1))
      printf '[failed] %s\n' "$file" >&2
      if [[ "$fail_fast" -eq 1 ]]; then
        break
      fi
    fi
  fi

  if [[ "$max_files" -gt 0 && "$matched_count" -ge "$max_files" ]]; then
    break
  fi
done < <(find "${find_args[@]}" | sort -z)

printf '[summary] scanned=%d matched=%d excluded=%d\n' \
  "$total_count" "$matched_count" "$excluded_count" >&2
if [[ "$upload" -eq 1 ]]; then
  printf '[upload-summary] uploaded=%d failed=%d\n' "$uploaded_count" "$failed_count" >&2
fi

if [[ -n "$output_file" ]]; then
  printf '[output] %s\n' "$output_file" >&2
fi
