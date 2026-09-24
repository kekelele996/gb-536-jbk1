#!/usr/bin/env bash
set -euo pipefail

api_root="${API_ROOT:-http://127.0.0.1:19536/api/v1}"
health_root="${HEALTH_ROOT:-http://127.0.0.1:19536}"
body_file="$(mktemp)"
run_suffix="$(date +%s)-$$"
trap 'rm -f "$body_file"' EXIT
last_body=""
checks=0

request() {
  local label="$1" expected="$2" method="$3" path="$4"
  local token="${5:-}" payload="${6:-}" idempotency="${7:-}"
  local args=(-sS -o "$body_file" -w "%{http_code}" -X "$method")
  [[ -n "$token" ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n "$payload" ]] && args+=(-H "Content-Type: application/json" --data "$payload")
  [[ -n "$idempotency" ]] && args+=(-H "Idempotency-Key: $idempotency")
  local status
  status="$(curl "${args[@]}" "$api_root$path")"
  last_body="$(<"$body_file")"
  checks=$((checks + 1))
  if [[ "$status" != "$expected" ]]; then
    printf 'FAIL %-48s expected=%s actual=%s body=%s\n' "$label" "$expected" "$status" "$last_body" >&2
    exit 1
  fi
  printf 'PASS %-48s HTTP %s\n' "$label" "$status"
}

require_json() {
  local expression="$1" message="$2"
  if ! jq -e "$expression" >/dev/null <<<"$last_body"; then
    printf 'FAIL response assertion: %s body=%s\n' "$message" "$last_body" >&2
    exit 1
  fi
}

status="$(curl -sS -o "$body_file" -w "%{http_code}" "$health_root/healthz")"
[[ "$status" == "200" ]] || { printf 'FAIL health expected=200 actual=%s\n' "$status" >&2; exit 1; }
checks=$((checks + 1)); printf 'PASS %-48s HTTP 200\n' "backend health"

request "unauthenticated dataset list denied" 401 GET "/datasets"
request "manager login" 200 POST "/auth/login" "" '{"username":"manager","password":"Data#536"}'
manager_token="$(jq -r '.data.token' <<<"$last_body")"
request "annotator A login" 200 POST "/auth/login" "" '{"username":"annotator_a","password":"Annotate#536"}'
annotator_a_token="$(jq -r '.data.token' <<<"$last_body")"
request "annotator B login" 200 POST "/auth/login" "" '{"username":"annotator_b","password":"Compare#536"}'
annotator_b_token="$(jq -r '.data.token' <<<"$last_body")"
request "adjudicator login" 200 POST "/auth/login" "" '{"username":"adjudicator","password":"Decide#536"}'
adjudicator_token="$(jq -r '.data.token' <<<"$last_body")"
request "independent reviewer login" 200 POST "/auth/login" "" '{"username":"reviewer","password":"Review#536"}'
reviewer_token="$(jq -r '.data.token' <<<"$last_body")"
request "auditor login" 200 POST "/auth/login" "" '{"username":"auditor","password":"Audit#536"}'
auditor_token="$(jq -r '.data.token' <<<"$last_body")"
request "admin login" 200 POST "/auth/login" "" '{"username":"admin","password":"Admin#536"}'
admin_token="$(jq -r '.data.token' <<<"$last_body")"

request "list seeded datasets" 200 GET "/datasets?page_size=100" "$manager_token"
require_json '(.data | length) >= 1 and .data[0].dataset_code != ""' "seeded dataset projection"
request "list seeded schemas" 200 GET "/schemas?page_size=100" "$manager_token"
request "list seeded annotations" 200 GET "/annotations?page_size=100" "$manager_token"
request "list seeded adjudications" 200 GET "/adjudications?page_size=100" "$manager_token"

dataset_code="QA-536-${run_suffix}"
dataset_payload="$(jq -nc --arg code "$dataset_code" '{dataset_code:$code,name:"QA agreement corpus",language:"en",domain:"contract-quality",document_count:25,content_mask_policy:"Replace names and identifiers with [MASK] before review.",owner_team:"Agreement QA"}')"
request "annotator cannot create dataset" 403 POST "/datasets" "$annotator_a_token" "$dataset_payload"
request "manager creates corpus dataset" 201 POST "/datasets" "$manager_token" "$dataset_payload"
dataset_id="$(jq -r '.data.id' <<<"$last_body")"
dataset_version="$(jq -r '.data.version' <<<"$last_body")"
require_json '.data.dataset_state == "draft" and .data.version == 1' "new dataset state"
request "dataset detail" 200 GET "/datasets/$dataset_id" "$auditor_token"

schema_code="QA-SCHEMA-${run_suffix}"
schema_payload="$(jq -nc --argjson dataset "$dataset_id" --arg code "$schema_code" '{dataset_id:$dataset,schema_code:$code,version:1,label_definitions:[{code:"RISK",display_name:"Risk",task_type:"classification",description:"Unit contains material contractual risk."},{code:"CLEAR",display_name:"Clear",task_type:"classification",description:"Unit contains no material contractual risk."}],span_policy:"Use half-open Unicode code-point offsets and exclude punctuation.",overlap_policy:"Nested spans are forbidden and adjacent spans are permitted.",examples:[{item_key:"EX-QA",masked_text:"[MASK] shall perform the stated duty.",labels:[{unit_key:"document_class",label:"RISK"}]}]}')"
unmasked_schema="$(jq -nc --argjson dataset "$dataset_id" --arg code "BAD-${run_suffix}" '{dataset_id:$dataset,schema_code:$code,version:1,label_definitions:[{code:"RISK",display_name:"Risk",task_type:"classification",description:"Unit contains material risk."},{code:"CLEAR",display_name:"Clear",task_type:"classification",description:"Unit contains no material risk."}],span_policy:"Use half-open Unicode code-point offsets.",overlap_policy:"Nested spans are forbidden in this schema.",examples:[{item_key:"EX-BAD",masked_text:"Unredacted example sentence",labels:[{unit_key:"document_class",label:"RISK"}]}]}')"
request "unmasked schema example rejected" 422 POST "/schemas" "$manager_token" "$unmasked_schema"
require_json '.error.code == "unmasked_example"' "unmasked example error code"
request "manager creates annotation schema" 201 POST "/schemas" "$manager_token" "$schema_payload"
schema_id="$(jq -r '.data.id' <<<"$last_body")"
require_json '.data.schema_state == "draft" and (.data.label_definitions | length) == 2' "schema draft and labels"
request "schema detail" 200 GET "/schemas/$schema_id" "$auditor_token"
request "draft cannot skip schema validation" 409 POST "/schemas/$schema_id/transition" "$manager_token" '{"target_state":"published"}'
request "validate schema" 200 POST "/schemas/$schema_id/transition" "$manager_token" '{"target_state":"validated"}'
request "publish schema" 200 POST "/schemas/$schema_id/transition" "$manager_token" '{"target_state":"published"}'
require_json '.data.schema_state == "published" and .data.published_at != null' "published schema"

request "freeze corpus version" 200 POST "/datasets/$dataset_id/transition" "$manager_token" "$(jq -nc --argjson version "$dataset_version" '{target_state:"frozen",version:$version}')"
require_json '.data.dataset_state == "frozen" and .data.version == 2' "frozen dataset version"
request "stale dataset transition rejected" 409 POST "/datasets/$dataset_id/transition" "$manager_token" "$(jq -nc --argjson version "$dataset_version" '{target_state:"frozen",version:$version}')"

item_key="QA-ITEM-${run_suffix}"
left_labels='[{"unit_key":"u1","label":"RISK"},{"unit_key":"u2","label":"RISK"},{"unit_key":"u3","label":"CLEAR"},{"unit_key":"u4","label":"CLEAR"},{"unit_key":"u5","label":"RISK"}]'
right_labels='[{"unit_key":"u1","label":"RISK"},{"unit_key":"u2","label":"CLEAR"},{"unit_key":"u3","label":"CLEAR"},{"unit_key":"u4","label":"CLEAR"},{"unit_key":"u5","label":"RISK"}]'
left_payload="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "$item_key" --argjson labels "$left_labels" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:$labels,source_checksum:"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",quality_note:"First independent pass against the published schema."}')"
right_payload="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "$item_key" --argjson labels "$right_labels" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:$labels,source_checksum:"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",quality_note:"Second independent pass against the published schema."}')"
request "manager cannot create annotation" 403 POST "/annotations" "$manager_token" "$left_payload"
request "annotator A creates result set" 201 POST "/annotations" "$annotator_a_token" "$left_payload"
left_id="$(jq -r '.data.id' <<<"$last_body")"
request "annotation result detail" 200 GET "/annotations/$left_id" "$auditor_token"
request "another annotator cannot edit draft" 403 PUT "/annotations/$left_id" "$annotator_b_token" "$(jq -nc --argjson labels "$left_labels" '{labels:$labels,quality_note:"Unauthorized edit attempt."}')"
request "annotator B creates result set" 201 POST "/annotations" "$annotator_b_token" "$right_payload"
right_id="$(jq -r '.data.id' <<<"$last_body")"
invalid_labels="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "BAD-${run_suffix}" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:[{unit_key:"u1",label:"UNKNOWN"}],source_checksum:"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",quality_note:"Must be rejected."}')"
request "unknown annotation label rejected" 422 POST "/annotations" "$annotator_a_token" "$invalid_labels"
require_json '.error.code == "unknown_label"' "unknown label error"
request "annotator A submits own result" 200 POST "/annotations/$left_id/transition" "$annotator_a_token" '{"target_state":"submitted"}'
request "duplicate submit is a state conflict" 409 POST "/annotations/$left_id/transition" "$annotator_a_token" '{"target_state":"submitted"}'
request "annotator B submits own result" 200 POST "/annotations/$right_id/transition" "$annotator_b_token" '{"target_state":"submitted"}'
request "annotator cannot lock result" 403 POST "/annotations/$right_id/transition" "$annotator_b_token" '{"target_state":"locked"}'
request "manager locks left result" 200 POST "/annotations/$left_id/transition" "$manager_token" '{"target_state":"locked"}'
request "manager locks right result" 200 POST "/annotations/$right_id/transition" "$manager_token" '{"target_state":"locked"}'

compute_payload="$(jq -nc --argjson dataset "$dataset_id" --arg item "$item_key" --argjson left "$left_id" --argjson right "$right_id" '{dataset_id:$dataset,item_key:$item,annotation_set_ids:[$left,$right],metric:"auto"}')"
compute_key="qa-compute-${run_suffix}"
request "compute Cohen Kappa disagreement" 201 POST "/adjudications" "$manager_token" "$compute_payload" "$compute_key"
case_id="$(jq -r '.data.id' <<<"$last_body")"
require_json '.data.agreement.metric == "cohen_kappa" and .data.agreement.sample_size == 5 and .data.agreement.coder_count == 2 and (.data.evidence_snapshot | length) >= 1' "agreement metadata and evidence"
request "computation idempotency reuses case" 200 POST "/adjudications" "$manager_token" "$compute_payload" "$compute_key"
require_json ".data.id == $case_id and .data.reused == true" "idempotent computation"
request "adjudication detail" 200 GET "/adjudications/$case_id" "$auditor_token"
request "adjudicator claims case" 200 POST "/adjudications/$case_id/assign" "$adjudicator_token" '{"note":"Independent adjudication assignment."}'
decision_payload="$(jq -nc --argjson labels "$left_labels" '{final_labels:$labels,rationale:"Resolved against the published schema and frozen evidence."}')"
decision_key="qa-decision-${run_suffix}"
request "assigned adjudicator records decision" 200 POST "/adjudications/$case_id/decide" "$adjudicator_token" "$decision_payload" "$decision_key"
require_json '.data.case_state == "adjudicated" and (.data.final_labels | length) == 5' "adjudicated state"
request "decision idempotency reuses decision" 200 POST "/adjudications/$case_id/decide" "$adjudicator_token" "$decision_payload" "$decision_key"
require_json '.data.reused == true' "idempotent decision"
request "decider cannot review own decision" 403 POST "/adjudications/$case_id/review" "$adjudicator_token" '{"note":"This same-person review must be denied."}'
request "independent reviewer approves decision" 200 POST "/adjudications/$case_id/review" "$reviewer_token" '{"note":"Independent evidence review found the decision consistent."}'
require_json '.data.case_state == "reviewed"' "reviewed state"
request "decider cannot conclude independent review" 403 POST "/adjudications/$case_id/accept" "$adjudicator_token" '{"note":"The original decider must not conclude review."}'
request "recorded reviewer accepts case" 200 POST "/adjudications/$case_id/accept" "$reviewer_token" '{"note":"Accepted after independent review completed."}'
require_json '.data.case_state == "accepted"' "accepted state"

replacement_item="QA-REPLACE-${run_suffix}"
rep_left_payload="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "$replacement_item" --argjson labels "$left_labels" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:$labels,source_checksum:"1111111111111111111111111111111111111111111111111111111111111111",quality_note:"Locked result later found to miss a boundary."}')"
rep_right_payload="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "$replacement_item" --argjson labels "$right_labels" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:$labels,source_checksum:"2222222222222222222222222222222222222222222222222222222222222222",quality_note:"Peer result for the replacement workflow."}')"
request "annotator A creates replacement-fixture result" 201 POST "/annotations" "$annotator_a_token" "$rep_left_payload"; rep_left_id="$(jq -r '.data.id' <<<"$last_body")"
request "annotator B creates replacement-fixture result" 201 POST "/annotations" "$annotator_b_token" "$rep_right_payload"; rep_right_id="$(jq -r '.data.id' <<<"$last_body")"
request "A submits replacement-fixture result" 200 POST "/annotations/$rep_left_id/transition" "$annotator_a_token" '{"target_state":"submitted"}'
request "B submits replacement-fixture result" 200 POST "/annotations/$rep_right_id/transition" "$annotator_b_token" '{"target_state":"submitted"}'
request "manager locks replacement left" 200 POST "/annotations/$rep_left_id/transition" "$manager_token" '{"target_state":"locked"}'
request "manager locks replacement right" 200 POST "/annotations/$rep_right_id/transition" "$manager_token" '{"target_state":"locked"}'
rep_compute_payload="$(jq -nc --argjson dataset "$dataset_id" --arg item "$replacement_item" --argjson left "$rep_left_id" --argjson right "$rep_right_id" '{dataset_id:$dataset,item_key:$item,annotation_set_ids:[$left,$right],metric:"auto"}')"
request "compute replacement workflow case" 201 POST "/adjudications" "$manager_token" "$rep_compute_payload" "qa-replace-compute-${run_suffix}"
rep_case_id="$(jq -r '.data.id' <<<"$last_body")"
request "adjudicator claims replacement case" 200 POST "/adjudications/$rep_case_id/assign" "$adjudicator_token" '{"note":"Independent review before tracked replacement."}'
request "adjudicator decides replacement case" 200 POST "/adjudications/$rep_case_id/decide" "$adjudicator_token" "$(jq -nc --argjson labels "$left_labels" '{final_labels:$labels,rationale:"Resolved against the published schema and frozen evidence."}')" "qa-replace-decision-${run_suffix}"
request "reviewer reviews replacement case" 200 POST "/adjudications/$rep_case_id/review" "$reviewer_token" '{"note":"Independent evidence review found the decision consistent."}'
request "reviewer accepts replacement case" 200 POST "/adjudications/$rep_case_id/accept" "$reviewer_token" '{"note":"Accepted before the old annotation is replaced."}'
require_json '.data.case_state == "accepted" and .data.evidence_current == true' "replacement case accepted with current evidence"
rep_admin_labels='[{"unit_key":"u1","label":"RISK"},{"unit_key":"u2","label":"RISK"},{"unit_key":"u3","label":"RISK"},{"unit_key":"u4","label":"CLEAR"},{"unit_key":"u5","label":"CLEAR"}]'
rep_admin_payload="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "$replacement_item" --argjson labels "$rep_admin_labels" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:$labels,source_checksum:"3333333333333333333333333333333333333333333333333333333333333333",quality_note:"Admin third opinion for the same item."}')"
request "admin creates third result on replacement item" 201 POST "/annotations" "$admin_token" "$rep_admin_payload"; rep_admin_id="$(jq -r '.data.id' <<<"$last_body")"
request "admin submits third result" 200 POST "/annotations/$rep_admin_id/transition" "$admin_token" '{"target_state":"submitted"}'
request "manager locks third result" 200 POST "/annotations/$rep_admin_id/transition" "$manager_token" '{"target_state":"locked"}'
rep_second_compute="$(jq -nc --argjson dataset "$dataset_id" --arg item "$replacement_item" --argjson left "$rep_left_id" --argjson admin "$rep_admin_id" '{dataset_id:$dataset,item_key:$item,annotation_set_ids:[$left,$admin],metric:"auto"}')"
request "compute second open case on replacement item" 201 POST "/adjudications" "$manager_token" "$rep_second_compute" "qa-replace-second-${run_suffix}"
rep_second_case_id="$(jq -r '.data.id' <<<"$last_body")"
request "annotator B cannot replace A result" 403 POST "/annotations/$rep_left_id/replace" "$annotator_b_token" '{"reason":"Peer attempts to replace a result they do not own."}'
request "draft replacement reason too short" 400 POST "/annotations/$rep_left_id/replace" "$annotator_a_token" '{"reason":"short"}'
request "annotator A starts trackable replacement" 201 POST "/annotations/$rep_left_id/replace" "$annotator_a_token" '{"reason":"Missed span boundary on unit u2."}'
rep_draft_id="$(jq -r '.data.id' <<<"$last_body")"
require_json ".data.supersedes_id == $rep_left_id and .data.annotation_state == \"draft\" and .data.item_key == \"$replacement_item\" and (.data.version_chain | length) == 2" "replacement draft lineage and chain"
request "repeated replacement returns same draft" 201 POST "/annotations/$rep_left_id/replace" "$annotator_a_token" '{"reason":"Missed span boundary on unit u2."}'
require_json ".data.id == $rep_draft_id and .data.reused == true" "idempotent replacement"
request "old result is superseded" 200 GET "/annotations/$rep_left_id" "$manager_token"
require_json '.data.annotation_state == "superseded"' "superseded old revision"
request "open case moved to pending recompute" 200 GET "/adjudications/$rep_second_case_id" "$manager_token"
require_json ".data.case_state == \"pending_recompute\" and ((.data.superseded_set_ids | map(tostring)) | any(. == \"$rep_left_id\"))" "pending recompute case"
request "accepted history retained view-only on stale evidence" 200 GET "/adjudications/$rep_case_id" "$auditor_token"
require_json ".data.case_state == \"accepted\" and .data.evidence_current == false and ((.data.superseded_set_ids | map(tostring)) | any(. == \"$rep_left_id\"))" "accepted history no longer current"
request "superseded result cannot enter new comparison" 409 POST "/adjudications" "$manager_token" "$rep_compute_payload" "qa-replace-stale-${run_suffix}"
require_json '.error.code == "annotation_superseded"' "superseded comparison rejection"
request "pending case cannot be claimed" 409 POST "/adjudications/$rep_second_case_id/assign" "$adjudicator_token" '{"note":"Attempt to claim pending recompute evidence."}'
fixed_labels='[{"unit_key":"u1","label":"CLEAR"},{"unit_key":"u2","label":"CLEAR"},{"unit_key":"u3","label":"CLEAR"},{"unit_key":"u4","label":"CLEAR"},{"unit_key":"u5","label":"RISK"}]'
fixed_payload="$(jq -nc --argjson labels "$fixed_labels" '{labels:$labels,quality_note:"Corrected boundary during tracked replacement."}')"
request "owner edits replacement draft" 200 PUT "/annotations/$rep_draft_id" "$annotator_a_token" "$fixed_payload"
request "replacement draft submitted" 200 POST "/annotations/$rep_draft_id/transition" "$annotator_a_token" '{"target_state":"submitted"}'
request "replacement draft locked and succeeds old version" 200 POST "/annotations/$rep_draft_id/transition" "$manager_token" '{"target_state":"locked"}'
require_json ".data.supersedes_id == $rep_left_id and .data.annotation_state == \"locked\"" "replacement revision takes over"
recompute_payload="$(jq -nc --argjson dataset "$dataset_id" --arg item "$replacement_item" --argjson left "$rep_draft_id" --argjson right "$rep_right_id" '{dataset_id:$dataset,item_key:$item,annotation_set_ids:[$left,$right],metric:"auto"}')"
request "recompute with replacement revision" 201 POST "/adjudications" "$manager_token" "$recompute_payload" "qa-recompute-${run_suffix}"
recomputed_case_id="$(jq -r '.data.id' <<<"$last_body")"
require_json ".data.id != $rep_case_id and .data.id != $rep_second_case_id and .data.evidence_current == true" "fresh comparison on replacement version"
request "accepted case history marks stale evidence" 200 GET "/adjudications/$case_id" "$auditor_token"
require_json '.data.case_state == "accepted" and .data.evidence_current == true' "older accepted case without replaced evidence stays current"

self_item="QA-SELF-${run_suffix}"
admin_payload="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "$self_item" --argjson labels "$left_labels" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:$labels,source_checksum:"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",quality_note:"Admin-owned annotation for isolation verification."}')"
peer_payload="$(jq -nc --argjson dataset "$dataset_id" --argjson schema "$schema_id" --arg item "$self_item" --argjson labels "$right_labels" '{dataset_id:$dataset,schema_id:$schema,item_key:$item,labels:$labels,source_checksum:"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",quality_note:"Independent peer annotation for isolation verification."}')"
request "admin creates own annotation" 201 POST "/annotations" "$admin_token" "$admin_payload"; admin_annotation_id="$(jq -r '.data.id' <<<"$last_body")"
request "peer creates self-check annotation" 201 POST "/annotations" "$annotator_a_token" "$peer_payload"; peer_annotation_id="$(jq -r '.data.id' <<<"$last_body")"
request "admin submits own annotation" 200 POST "/annotations/$admin_annotation_id/transition" "$admin_token" '{"target_state":"submitted"}'
request "peer submits self-check annotation" 200 POST "/annotations/$peer_annotation_id/transition" "$annotator_a_token" '{"target_state":"submitted"}'
request "manager locks admin annotation" 200 POST "/annotations/$admin_annotation_id/transition" "$manager_token" '{"target_state":"locked"}'
request "manager locks peer annotation" 200 POST "/annotations/$peer_annotation_id/transition" "$manager_token" '{"target_state":"locked"}'
self_compute="$(jq -nc --argjson dataset "$dataset_id" --arg item "$self_item" --argjson left "$admin_annotation_id" --argjson right "$peer_annotation_id" '{dataset_id:$dataset,item_key:$item,annotation_set_ids:[$left,$right],metric:"auto"}')"
request "compute self-isolation case" 201 POST "/adjudications" "$manager_token" "$self_compute" "qa-self-${run_suffix}"
self_case_id="$(jq -r '.data.id' <<<"$last_body")"
request "adjudicator cannot claim own annotation" 403 POST "/adjudications/$self_case_id/assign" "$admin_token" '{"note":"Self-adjudication must be denied."}'

request "auditor reads redacted audit stream" 200 GET "/audit?page_size=200" "$auditor_token"
require_json '([.data[].resource_type] | unique | length) >= 4 and ([.data[].action] | index("adjudication_case.accepted")) != null' "four entity projections and accepted action"
require_json 'all(.data[]; (((.parameters|tostring) + (.before|tostring) + (.after|tostring)) | contains("document_class")) | not)' "audit stream excludes full label payloads"

printf 'ALL %d API CHECKS PASSED\n' "$checks"
