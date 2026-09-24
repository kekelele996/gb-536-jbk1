import { inject, Injectable, signal } from '@angular/core';
import { finalize, Observable } from 'rxjs';
import { AnnotationSchemaApi } from '../api/annotation-schema';
import { ApiEnvelope } from '../types/api';
import { AnnotationSchema, CreateAnnotationSchema, UpdateAnnotationSchema } from '../types/annotation-schema';
import { SchemaState } from '../types/enums/annotation-state';
import { apiErrorMessage } from '../utils/api-error';

@Injectable({ providedIn: 'root' })
export class AnnotationSchemaStore {
  private readonly api = inject(AnnotationSchemaApi);
  readonly items = signal<AnnotationSchema[]>([]);
  readonly selected = signal<AnnotationSchema | null>(null);
  readonly loading = signal(false);
  readonly error = signal('');

  load(datasetId?: number): void {
    this.loading.set(true);
    this.error.set('');
    this.api.list(datasetId).pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.set(data);
        const selected = this.selected();
        this.selected.set(data.find((item) => item.id === selected?.id) ?? data[0] ?? null);
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }

  choose(schema: AnnotationSchema): void {
    this.selected.set(schema);
  }

  create(payload: CreateAnnotationSchema, done?: () => void): void {
    this.mutate(this.api.create(payload), done);
  }

  update(id: number, payload: UpdateAnnotationSchema, done?: () => void): void {
    this.mutate(this.api.update(id, payload), done);
  }

  copy(schema: AnnotationSchema, version: number): void {
    this.mutate(this.api.copy(schema.id, version));
  }

  transition(schema: AnnotationSchema, target: SchemaState): void {
    this.mutate(this.api.transition(schema.id, target));
  }

  private mutate(request: Observable<ApiEnvelope<AnnotationSchema>>, done?: () => void): void {
    this.loading.set(true);
    this.error.set('');
    request.pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.update((items) =>
          [data, ...items.filter((item) => item.id !== data.id)]
            .sort((left, right) => left.schema_code.localeCompare(right.schema_code) || right.version - left.version),
        );
        this.selected.set(data);
        done?.();
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
}
