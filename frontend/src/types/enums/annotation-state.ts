export type AnnotationState = 'draft' | 'submitted' | 'returned' | 'locked' | 'compared' | 'superseded';
export type DatasetState = 'draft' | 'frozen' | 'archived';
export type SchemaState = 'draft' | 'validated' | 'published' | 'deprecated';
export type CaseState = 'open' | 'assigned' | 'adjudicated' | 'reviewed' | 'accepted' | 'reopened';

export const ANNOTATION_STATES: readonly AnnotationState[] = [
  'draft', 'submitted', 'returned', 'locked', 'compared', 'superseded',
];
