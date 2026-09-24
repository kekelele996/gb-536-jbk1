import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { CreateCorpusDataset, CorpusDataset, UpdateCorpusDataset } from '../types/corpus-dataset';
import { DatasetState } from '../types/enums/annotation-state';

@Injectable({ providedIn: 'root' })
export class CorpusDatasetApi {
  private readonly api = inject(ApiClient);

  list() {
    return this.api.page<CorpusDataset>('/datasets', { page_size: 100 });
  }

  get(id: number) {
    return this.api.get<CorpusDataset>(`/datasets/${id}`);
  }

  create(payload: CreateCorpusDataset) {
    return this.api.post<CorpusDataset>('/datasets', payload);
  }

  update(id: number, payload: UpdateCorpusDataset) {
    return this.api.put<CorpusDataset>(`/datasets/${id}`, payload);
  }

  transition(id: number, targetState: Extract<DatasetState, 'frozen' | 'archived'>, version: number) {
    return this.api.post<CorpusDataset>(`/datasets/${id}/transition`, { target_state: targetState, version });
  }
}
