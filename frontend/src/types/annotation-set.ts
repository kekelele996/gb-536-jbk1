import { AnnotationState } from './enums/annotation-state';

export interface AnnotationLabel {
  unit_key: string;
  label: string;
  start?: number;
  end?: number;
}

export interface AnnotationVersionChainItem {
  id: number;
  annotation_state: AnnotationState;
  supersedes_id?: number;
  replace_reason: string;
  quality_note: string;
  submitted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AnnotationSet {
  id: number;
  dataset_id: number;
  dataset_code: string;
  schema_id: number;
  schema_code: string;
  schema_version: number;
  annotator_id: number;
  annotator: string;
  item_key: string;
  labels: AnnotationLabel[];
  source_checksum: string;
  annotation_state: AnnotationState;
  submitted_at?: string;
  supersedes_id?: number;
  replace_reason?: string;
  quality_note: string;
  version_chain?: AnnotationVersionChainItem[];
  created_at: string;
  updated_at: string;
  reused?: boolean;
}

export interface CreateAnnotationSet {
  dataset_id: number;
  schema_id: number;
  item_key: string;
  labels: AnnotationLabel[];
  source_checksum: string;
  quality_note: string;
}

export interface UpdateAnnotationSet {
  labels: AnnotationLabel[];
  quality_note: string;
}
