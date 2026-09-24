import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { AnnotationSet, AnnotationVersionChain, CreateAnnotationSet, ReplaceAnnotationSet, UpdateAnnotationSet } from '../types/annotation-set';
import { AnnotationState } from '../types/enums/annotation-state';

@Injectable({ providedIn: 'root' })
export class AnnotationSetApi {
  private readonly api = inject(ApiClient);

  list(datasetId?: number, itemKey?: string) {
    return this.api.page<AnnotationSet>('/annotations', { page_size: 150, dataset_id: datasetId, item_key: itemKey });
  }

  get(id: number) {
    return this.api.get<AnnotationSet>(`/annotations/${id}`);
  }

  versionChain(datasetId: number, itemKey: string, annotatorId: number) {
    return this.api.get<AnnotationVersionChain>('/annotations/version-chain', {
      dataset_id: datasetId, item_key: itemKey, annotator_id: annotatorId,
    });
  }

  create(payload: CreateAnnotationSet) {
    return this.api.post<AnnotationSet>('/annotations', payload);
  }

  update(id: number, payload: UpdateAnnotationSet) {
    return this.api.put<AnnotationSet>(`/annotations/${id}`, payload);
  }

  replace(id: number, payload: ReplaceAnnotationSet) {
    return this.api.post<AnnotationSet>(`/annotations/${id}/replace`, payload);
  }

  transition(id: number, targetState: AnnotationState, reason = '') {
    return this.api.post<AnnotationSet>(`/annotations/${id}/transition`, { target_state: targetState, reason });
  }
}
