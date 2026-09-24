import { inject, Injectable, signal } from '@angular/core';
import { finalize, Observable } from 'rxjs';
import { AnnotationSetApi } from '../api/annotation-set';
import { ApiEnvelope } from '../types/api';
import { AnnotationSet, CreateAnnotationSet, UpdateAnnotationSet } from '../types/annotation-set';
import { AnnotationState } from '../types/enums/annotation-state';
import { apiErrorMessage } from '../utils/api-error';

@Injectable({ providedIn: 'root' })
export class AnnotationSetStore {
  private readonly api = inject(AnnotationSetApi);
  readonly items = signal<AnnotationSet[]>([]);
  readonly selected = signal<AnnotationSet | null>(null);
  readonly loading = signal(false);
  readonly error = signal('');

  private lastDatasetId: number | undefined;
  private lastItemKey: string | undefined;

  load(datasetId?: number, itemKey?: string): void {
    this.lastDatasetId = datasetId;
    this.lastItemKey = itemKey;
    this.loading.set(true);
    this.error.set('');
    this.api.list(datasetId, itemKey).pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.set(data);
        const selected = this.selected();
        this.selected.set(data.find((item) => item.id === selected?.id) ?? data[0] ?? null);
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }

  choose(annotation: AnnotationSet): void {
    this.selected.set(annotation);
    // The list projection does not embed the version chain; hydrate it from
    // the detail endpoint so the selected record can render its lineage.
    if (!annotation.version_chain?.length) {
      this.api.get(annotation.id).subscribe({
        next: ({ data }) => this.merge(data),
        error: () => undefined,
      });
    }
  }

  create(payload: CreateAnnotationSet, done?: () => void): void {
    this.mutate(this.api.create(payload), done);
  }

  update(id: number, payload: UpdateAnnotationSet, done?: () => void): void {
    this.mutate(this.api.update(id, payload), done);
  }

  replace(annotation: AnnotationSet, reason: string, done?: () => void): void {
    this.mutate(this.api.replace(annotation.id, reason), done, true);
  }

  transition(annotation: AnnotationSet, target: AnnotationState, reason = ''): void {
    this.mutate(this.api.transition(annotation.id, target, reason), undefined, true);
  }

  private merge(data: AnnotationSet): void {
    this.items.update((items) => {
      const without = items.filter((item) => item.id !== data.id);
      const index = items.findIndex((item) => item.id === data.id);
      if (index >= 0) {
        without.splice(index, 0, data);
        return without;
      }
      return [data, ...without];
    });
    this.selected.set(data);
  }

  private mutate(request: Observable<ApiEnvelope<AnnotationSet>>, done?: () => void, reloadList = false): void {
    this.loading.set(true);
    this.error.set('');
    request.pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.merge(data);
        // A replacement retires old rows and changes adjudication state;
        // refresh the register so stale versions disappear from actions.
        if (reloadList) {
          this.api.list(this.lastDatasetId, this.lastItemKey ?? data.item_key).subscribe({
            next: ({ data: items }) => {
              this.items.set(items);
              this.selected.set(items.find((item) => item.id === data.id) ?? data);
            },
            error: () => undefined,
          });
        }
        done?.();
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
}
