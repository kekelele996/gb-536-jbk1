import { inject, Injectable, signal } from '@angular/core';
import { finalize, Observable } from 'rxjs';
import { CorpusDatasetApi } from '../api/corpus-dataset';
import { ApiEnvelope } from '../types/api';
import { CreateCorpusDataset, CorpusDataset, UpdateCorpusDataset } from '../types/corpus-dataset';
import { apiErrorMessage } from '../utils/api-error';

@Injectable({ providedIn: 'root' })
export class CorpusDatasetStore {
  private readonly api = inject(CorpusDatasetApi);
  readonly items = signal<CorpusDataset[]>([]);
  readonly selected = signal<CorpusDataset | null>(null);
  readonly loading = signal(false);
  readonly error = signal('');

  load(): void {
    this.loading.set(true);
    this.error.set('');
    this.api.list().pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.set(data);
        const selected = this.selected();
        this.selected.set(data.find((item) => item.id === selected?.id) ?? data[0] ?? null);
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }

  choose(dataset: CorpusDataset): void {
    this.selected.set(dataset);
  }

  create(payload: CreateCorpusDataset, done?: () => void): void {
    this.mutate(this.api.create(payload), done);
  }

  update(id: number, payload: UpdateCorpusDataset, done?: () => void): void {
    this.mutate(this.api.update(id, payload), done);
  }

  transition(dataset: CorpusDataset, target: 'frozen' | 'archived'): void {
    this.mutate(this.api.transition(dataset.id, target, dataset.version));
  }

  private mutate(request: Observable<ApiEnvelope<CorpusDataset>>, done?: () => void): void {
    this.loading.set(true);
    this.error.set('');
    request.pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.update((items) =>
          [data, ...items.filter((item) => item.id !== data.id)]
            .sort((left, right) => left.dataset_code.localeCompare(right.dataset_code)),
        );
        this.selected.set(data);
        done?.();
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
}
