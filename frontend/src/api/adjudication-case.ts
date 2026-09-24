import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { AdjudicationCase, ComputeAdjudication } from '../types/adjudication-case';
import { AnnotationLabel } from '../types/annotation-set';

@Injectable({ providedIn: 'root' })
export class AdjudicationCaseApi {
  private readonly api = inject(ApiClient);

  list(datasetId?: number) {
    return this.api.page<AdjudicationCase>('/adjudications', { page_size: 150, dataset_id: datasetId });
  }

  get(id: number) {
    return this.api.get<AdjudicationCase>(`/adjudications/${id}`);
  }

  compute(payload: ComputeAdjudication, idempotencyKey: string) {
    return this.api.post<AdjudicationCase>('/adjudications', payload, { 'Idempotency-Key': idempotencyKey });
  }

  assign(id: number, note: string) {
    return this.api.post<AdjudicationCase>(`/adjudications/${id}/assign`, { note });
  }

  decide(id: number, finalLabels: AnnotationLabel[], rationale: string, idempotencyKey: string) {
    return this.api.post<AdjudicationCase>(
      `/adjudications/${id}/decide`,
      { final_labels: finalLabels, rationale },
      { 'Idempotency-Key': idempotencyKey },
    );
  }

  review(id: number, note: string) {
    return this.api.post<AdjudicationCase>(`/adjudications/${id}/review`, { note });
  }

  accept(id: number, note: string) {
    return this.api.post<AdjudicationCase>(`/adjudications/${id}/accept`, { note });
  }

  reopen(id: number, note: string) {
    return this.api.post<AdjudicationCase>(`/adjudications/${id}/reopen`, { note });
  }
}
