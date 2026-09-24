import { SchemaState } from './enums/annotation-state';
import { AnnotationLabel } from './annotation-set';

export interface LabelDefinition {
  code: string;
  display_name: string;
  task_type: 'classification' | 'span';
  description: string;
}

export interface MaskedExample {
  item_key: string;
  masked_text: string;
  labels: AnnotationLabel[];
}

export interface AnnotationSchema {
  id: number;
  dataset_id: number;
  dataset_code: string;
  schema_code: string;
  version: number;
  label_definitions: LabelDefinition[];
  span_policy: string;
  overlap_policy: string;
  examples: MaskedExample[];
  schema_state: SchemaState;
  created_by: number;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateAnnotationSchema {
  dataset_id: number;
  schema_code: string;
  version: number;
  label_definitions: LabelDefinition[];
  span_policy: string;
  overlap_policy: string;
  examples: MaskedExample[];
}

export interface UpdateAnnotationSchema {
  version: number;
  label_definitions: LabelDefinition[];
  span_policy: string;
  overlap_policy: string;
  examples: MaskedExample[];
}
