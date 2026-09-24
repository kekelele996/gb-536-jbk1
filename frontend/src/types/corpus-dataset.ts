import { DatasetState } from './enums/annotation-state';

export interface CorpusDataset {
  id: number;
  dataset_code: string;
  name: string;
  language: string;
  domain: string;
  document_count: number;
  content_mask_policy: string;
  dataset_state: DatasetState;
  version: number;
  owner_team: string;
  schema_count: number;
  annotation_count: number;
  created_at: string;
  updated_at: string;
}

export interface CreateCorpusDataset {
  dataset_code: string;
  name: string;
  language: string;
  domain: string;
  document_count: number;
  content_mask_policy: string;
  owner_team: string;
}

export interface UpdateCorpusDataset extends Omit<CreateCorpusDataset, 'dataset_code'> {
  version: number;
}
