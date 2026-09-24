import { inject, Injectable, signal } from '@angular/core';
import { finalize, Observable } from 'rxjs';
import { AdjudicationCaseApi } from '../api/adjudication-case';
import { ApiEnvelope } from '../types/api';
import { AdjudicationCase, ComputeAdjudication } from '../types/adjudication-case';
import { AnnotationLabel } from '../types/annotation-set';
import { apiErrorMessage } from '../utils/api-error';

@Injectable({ providedIn: 'root' })
export class AdjudicationCaseStore {
  private readonly api = inject(AdjudicationCaseApi);
  readonly items = signal<AdjudicationCase[]>([]);
  readonly selected = signal<AdjudicationCase | null>(null);
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

  choose(adjudication: AdjudicationCase): void {
    this.selected.set(adjudication);
  }

  compute(payload: ComputeAdjudication): void {
    this.mutate(this.api.compute(payload, `ui-compute-${crypto.randomUUID()}`));
  }

  assign(adjudication: AdjudicationCase, note: string): void {
    this.mutate(this.api.assign(adjudication.id, note));
  }

  decide(adjudication: AdjudicationCase, labels: AnnotationLabel[], rationale: string): void {
    this.mutate(this.api.decide(adjudication.id, labels, rationale, `ui-decision-${crypto.randomUUID()}`));
  }

  review(adjudication: AdjudicationCase, note: string): void {
    this.mutate(this.api.review(adjudication.id, note));
  }

  accept(adjudication: AdjudicationCase, note: string): void {
    this.mutate(this.api.accept(adjudication.id, note));
  }

  reopen(adjudication: AdjudicationCase, note: string): void {
    this.mutate(this.api.reopen(adjudication.id, note));
  }

  private mutate(request: Observable<ApiEnvelope<AdjudicationCase>>): void {
    this.loading.set(true);
    this.error.set('');
    request.pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => {
        this.items.update((items) => [data, ...items.filter((item) => item.id !== data.id)]);
        this.selected.set(data);
      },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
}
