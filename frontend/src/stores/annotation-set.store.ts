import { inject, Injectable, signal } from '@angular/core';
import { finalize, Observable } from 'rxjs';
import { AnnotationSetApi } from '../api/annotation-set';
import { ApiEnvelope } from '../types/api';
import { AnnotationSet, AnnotationVersionChain, CreateAnnotationSet, UpdateAnnotationSet } from '../types/annotation-set';
import { AnnotationState } from '../types/enums/annotation-state';
import { apiErrorMessage } from '../utils/api-error';

@Injectable({ providedIn: 'root' })
export class AnnotationSetStore {
  private readonly api = inject(AnnotationSetApi);
  readonly items = signal<AnnotationSet[]>([]);
  readonly selected = signal<AnnotationSet | null>(null);
  readonly versionChain = signal<AnnotationVersionChain | null>(null);
  readonly loading = signal(false);
  readonly error = signal('');

  load(datasetId?: number, itemKey?: string): void {
    this.loading.set(true);
    this.error.set('');
    this.api.list(datasetId, itemKey).pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.set(data);
        const selected = this.selected();
        this.selected.set(data.find((item) => item.id === selected?.id) ?? data[0] ?? null);
        this.loadChainFor(this.selected());
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }

  choose(annotation: AnnotationSet): void {
    this.selected.set(annotation);
    this.loadChainFor(annotation);
  }

  loadChainFor(annotation: AnnotationSet | null | undefined): void {
    if (!annotation) {
      this.versionChain.set(null);
      return;
    }
    this.api.versionChain(annotation.dataset_id, annotation.item_key, annotation.annotator_id).subscribe({
      next: ({ data }) => this.versionChain.set(data),
      error: () => this.versionChain.set(null),
    });
  }

  create(payload: CreateAnnotationSet, done?: () => void): void {
    this.mutate(this.api.create(payload), done);
  }

  update(id: number, payload: UpdateAnnotationSet, done?: () => void): void {
    this.mutate(this.api.update(id, payload), done);
  }

  replace(annotation: AnnotationSet, reason: string, done?: () => void): void {
    this.mutate(this.api.replace(annotation.id, { reason }), done);
  }

  transition(annotation: AnnotationSet, target: AnnotationState, reason = ''): void {
    this.mutate(this.api.transition(annotation.id, target, reason));
  }

  private mutate(request: Observable<ApiEnvelope<AnnotationSet>>, done?: () => void): void {
    this.loading.set(true);
    this.error.set('');
    request.pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.update((items) => [data, ...items.filter((item) => item.id !== data.id)]);
        this.selected.set(data);
        this.loadChainFor(data);
        done?.();
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
}
