import { AnnotationState } from './enums/annotation-state';

export interface AnnotationLabel {
  unit_key: string;
  label: string;
  start?: number;
  end?: number;
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
  superseded_by_id?: number;
  replacement_reason: string;
  quality_note: string;
  current_version: boolean;
  created_at: string;
  updated_at: string;
}

export interface AnnotationVersionLink {
  id: number;
  annotation_state: AnnotationState;
  labels: AnnotationLabel[];
  source_checksum: string;
  supersedes_id?: number;
  replacement_reason: string;
  quality_note: string;
  submitted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AnnotationVersionChain {
  dataset_id: number;
  item_key: string;
  annotator_id: number;
  annotator: string;
  links: AnnotationVersionLink[];
}

export interface CreateAnnotationSet {
  dataset_id: number;
  schema_id: number;
  item_key: string;
  labels: AnnotationLabel[];
  source_checksum: string;
  quality_note: string;
  supersedes_id?: number;
}

export interface UpdateAnnotationSet {
  labels: AnnotationLabel[];
  quality_note: string;
}

export interface ReplaceAnnotationSet {
  reason: string;
}
