import { inject } from '@angular/core';
import { AdjudicationCaseStore } from '../stores/adjudication-case.store';

export function useAdjudicationQueue(): AdjudicationCaseStore {
  return inject(AdjudicationCaseStore);
}
