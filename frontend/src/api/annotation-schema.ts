import { inject, Injectable } from '@angular/core';
import { ApiClient } from './api-client';
import { AnnotationSchema, CreateAnnotationSchema, UpdateAnnotationSchema } from '../types/annotation-schema';
import { SchemaState } from '../types/enums/annotation-state';

@Injectable({ providedIn: 'root' })
export class AnnotationSchemaApi {
  private readonly api = inject(ApiClient);

  list(datasetId?: number) {
    return this.api.page<AnnotationSchema>('/schemas', { page_size: 150, dataset_id: datasetId });
  }

  get(id: number) {
    return this.api.get<AnnotationSchema>(`/schemas/${id}`);
  }

  create(payload: CreateAnnotationSchema) {
    return this.api.post<AnnotationSchema>('/schemas', payload);
  }

  update(id: number, payload: UpdateAnnotationSchema) {
    return this.api.put<AnnotationSchema>(`/schemas/${id}`, payload);
  }

  copy(id: number, version: number) {
    return this.api.post<AnnotationSchema>(`/schemas/${id}/copy`, { version });
  }

  transition(id: number, targetState: SchemaState) {
    return this.api.post<AnnotationSchema>(`/schemas/${id}/transition`, { target_state: targetState });
  }
}
