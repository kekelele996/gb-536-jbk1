import { of, Subject } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { bindDatasetDetails } from './dataset-detail-loader';

describe('bindDatasetDetails', () => {
  it('clears both projections without issuing requests for an empty selection', () => {
    const loadSchemas = vi.fn(() => of({ data: ['unused'] }));
    const loadCases = vi.fn(() => of({ data: ['unused'] }));
    let schemas = ['stale'];
    let cases = ['stale'];

    const cleanup = bindDatasetDetails(
      null, loadSchemas, loadCases,
      (items) => { schemas = items; },
      (items) => { cases = items; },
    );

    expect(schemas).toEqual([]);
    expect(cases).toEqual([]);
    expect(loadSchemas).not.toHaveBeenCalled();
    expect(loadCases).not.toHaveBeenCalled();
    expect(cleanup()).toBeUndefined();
  });

  it('unsubscribes stale selection requests before the next selection resolves', () => {
    const firstSchemas = new Subject<{ data: string[] }>();
    const firstCases = new Subject<{ data: string[] }>();
    const secondSchemas = new Subject<{ data: string[] }>();
    const secondCases = new Subject<{ data: string[] }>();
    const schemas: string[][] = [];
    const cases: string[][] = [];
    const loadSchemas = vi.fn((id: number) => id === 1 ? firstSchemas : secondSchemas);
    const loadCases = vi.fn((id: number) => id === 1 ? firstCases : secondCases);

    const cleanupFirst = bindDatasetDetails(1, loadSchemas, loadCases, (items) => schemas.push(items), (items) => cases.push(items));
    cleanupFirst();
    const cleanupSecond = bindDatasetDetails(2, loadSchemas, loadCases, (items) => schemas.push(items), (items) => cases.push(items));

    firstSchemas.next({ data: ['stale-schema'] });
    firstCases.next({ data: ['stale-case'] });
    secondSchemas.next({ data: ['current-schema'] });
    secondCases.next({ data: ['current-case'] });
    cleanupSecond();

    expect(loadSchemas).toHaveBeenNthCalledWith(1, 1);
    expect(loadSchemas).toHaveBeenNthCalledWith(2, 2);
    expect(schemas).toEqual([['current-schema']]);
    expect(cases).toEqual([['current-case']]);
  });
});
