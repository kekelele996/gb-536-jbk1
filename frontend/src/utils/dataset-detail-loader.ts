import { Observable, Subscription } from 'rxjs';

interface ListResponse<T> { data: T[]; }

export function bindDatasetDetails<TSchema, TCase>(
  datasetId: number | null | undefined,
  loadSchemas: (datasetId: number) => Observable<ListResponse<TSchema>>,
  loadCases: (datasetId: number) => Observable<ListResponse<TCase>>,
  setSchemas: (items: TSchema[]) => void,
  setCases: (items: TCase[]) => void,
): () => void {
  if (!datasetId) {
    setSchemas([]);
    setCases([]);
    return () => undefined;
  }

  let active = true;
  const subscriptions = new Subscription();
  subscriptions.add(loadSchemas(datasetId).subscribe(({ data }) => {
    if (active) setSchemas(data);
  }));
  subscriptions.add(loadCases(datasetId).subscribe(({ data }) => {
    if (active) setCases(data);
  }));

  return () => {
    active = false;
    subscriptions.unsubscribe();
  };
}
