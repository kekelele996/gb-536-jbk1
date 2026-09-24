import { AnnotationLabel } from './annotation-set';
import { CaseState } from './enums/annotation-state';
import { DisagreementType } from './enums/disagreement-type';

export interface AgreementDetails {
  metric: 'cohen_kappa' | 'krippendorff_alpha';
  score: number;
  observed_agreement: number;
  chance_agreement: number;
  sample_size: number;
  coder_count: number;
  missing_value_count: number;
  applicability: string;
}

export interface ConfusionCell {
  left_label: string;
  right_label: string;
  count: number;
}

export interface DiffEvidence {
  type: DisagreementType;
  unit_key: string;
  left_label?: string;
  right_label?: string;
  left_start?: number;
  left_end?: number;
  right_start?: number;
  right_end?: number;
  overlap?: number;
  evidence: string;
}

export interface AdjudicationCase {
  id: number;
  dataset_id: number;
  dataset_code: string;
  item_key: string;
  annotation_set_ids: number[];
  agreement: AgreementDetails;
  disagreement_type: DisagreementType;
  confusion_snapshot: ConfusionCell[];
  evidence_snapshot: DiffEvidence[];
  cluster_key: string;
  case_state: CaseState;
  final_labels: AnnotationLabel[];
  rationale: string;
  adjudicator_id?: number;
  reviewed_by?: number;
  decided_at?: string;
  input_hash: string;
  algorithm_version: string;
  idempotency_key: string;
  decision_idempotency_key?: string;
  reopen_count: number;
  created_by: number;
  created_at: string;
  updated_at: string;
  reused: boolean;
}

export interface ComputeAdjudication {
  dataset_id: number;
  item_key: string;
  annotation_set_ids: number[];
  metric: 'auto' | 'cohen_kappa' | 'krippendorff_alpha';
}

export interface AuditEvent {
  id: number;
  actor: string;
  role: string;
  action: string;
  resource_type: string;
  resource_id: string;
  request_id: string;
  parameters: Record<string, unknown>;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
  created_at: string;
}
