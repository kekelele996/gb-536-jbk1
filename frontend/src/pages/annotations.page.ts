import { ChangeDetectionStrategy, Component, computed, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { FormsModule, FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { LucideAngularModule } from 'lucide-angular';
import { AnnotationStateBadgeComponent } from '../components/common/annotation-state-badge.component';
import { DiffEvidenceDrawerComponent } from '../components/common/diff-evidence-drawer.component';
import { useAuth } from '../hooks/use-auth';
import { AnnotationSchemaStore } from '../stores/annotation-schema.store';
import { AnnotationSetStore } from '../stores/annotation-set.store';
import { CorpusDatasetStore } from '../stores/corpus-dataset.store';
import { DiffEvidence } from '../types/adjudication-case';
import { AnnotationLabel, AnnotationSet, AnnotationVersionLink } from '../types/annotation-set';

@Component({
  standalone: true,
  imports: [DatePipe, FormsModule, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, LucideAngularModule, AnnotationStateBadgeComponent, DiffEvidenceDrawerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head"><div><span>Multi-annotator evidence</span><h1>Annotation comparison</h1></div><div class="head-actions"><button mat-stroked-button type="button" (click)="reload()" [disabled]="annotations.loading()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button>@if (canCreate()) { <button mat-flat-button color="primary" type="button" (click)="openCreate()" [disabled]="annotations.loading()"><lucide-icon name="plus" [size]="16" />New annotation</button> }</div></section>
    @if (annotations.error() || editorError()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ editorError() || annotations.error() }}</p> }
    <section class="filter-band">
      <mat-form-field appearance="outline"><mat-label>Dataset</mat-label><mat-select [(ngModel)]="datasetFilter" (selectionChange)="applyFilters()"><mat-option [value]="null">All datasets</mat-option>@for (dataset of datasets.items(); track dataset.id) { <mat-option [value]="dataset.id">{{ dataset.dataset_code }}</mat-option> }</mat-select></mat-form-field>
      <mat-form-field appearance="outline"><mat-label>Item key</mat-label><input matInput [(ngModel)]="itemFilter" (keyup.enter)="applyFilters()" /></mat-form-field>
      <button mat-flat-button color="primary" type="button" (click)="applyFilters()" [disabled]="annotations.loading()"><lucide-icon name="filter" [size]="15" />Apply</button>
    </section>
    <section class="annotation-layout">
      <div class="register">
        <header><span>Result sets</span><small>{{ annotations.items().length }} records</small></header>
        @for (annotation of annotations.items(); track annotation.id) {
          <button type="button" class="annotation-row" [class.selected]="annotations.selected()?.id === annotation.id" (click)="annotations.choose(annotation)">
            <span><strong>{{ annotation.item_key }}</strong><small>{{ annotation.annotator }} / {{ annotation.schema_code }} v{{ annotation.schema_version }}</small></span><span class="label-count">{{ annotation.labels.length }}</span><app-annotation-state-badge [state]="annotation.annotation_state" />
          </button>
        } @empty { <p class="empty">No annotation sets match the filter</p> }
        @if (annotations.selected(); as annotation) {
          <section class="selected-actions"><span>Selected #{{ annotation.id }} / {{ annotation.updated_at | date:'short' }}</span><div>@if (canEdit(annotation)) { <button mat-stroked-button type="button" (click)="openEdit(annotation)" [disabled]="annotations.loading()">Edit draft</button><button mat-flat-button color="primary" type="button" (click)="annotations.transition(annotation, 'submitted')" [disabled]="annotations.loading()">Submit</button> } @if (owns(annotation) && annotation.annotation_state === 'returned') { <button mat-flat-button color="primary" type="button" (click)="annotations.transition(annotation, 'draft')" [disabled]="annotations.loading()">Resume draft</button> } @if (canManage() && annotation.annotation_state === 'submitted') { <button mat-stroked-button type="button" (click)="annotations.transition(annotation, 'returned', 'Needs correction against the published schema.')" [disabled]="annotations.loading()">Return</button><button mat-flat-button color="primary" type="button" (click)="annotations.transition(annotation, 'locked')" [disabled]="annotations.loading()">Lock</button> } @if (canReplace(annotation)) { <button mat-flat-button color="accent" type="button" (click)="openReplace(annotation)" [disabled]="annotations.loading()"><lucide-icon name="copy" [size]="15" />Start replacement</button> }</div>@if (annotation.superseded_by_id) { <p class="retired-note"><lucide-icon name="archive" [size]="14" />Superseded by revision #{{ annotation.superseded_by_id }} — read-only history, excluded from new comparisons.</p> } @if (annotation.supersedes_id && annotation.replacement_reason) { <p class="replace-reason"><lucide-icon name="git-branch" [size]="14" />Replaces #{{ annotation.supersedes_id }}: {{ annotation.replacement_reason }}</p> }</section>
        }
      </div>
      <section class="compare-panel">
        <header><div><span>Side-by-side review</span><h2>{{ comparisonKey() }}</h2></div><small>No source text exposed</small></header>
        <div class="compare-selectors"><mat-form-field appearance="outline"><mat-label>Left result</mat-label><mat-select [value]="left()?.id" (selectionChange)="leftId.set($event.value)">@for (annotation of annotations.items(); track annotation.id) { <mat-option [value]="annotation.id">#{{ annotation.id }} / {{ annotation.annotator }}</mat-option> }</mat-select></mat-form-field><mat-form-field appearance="outline"><mat-label>Right result</mat-label><mat-select [value]="right()?.id" (selectionChange)="rightId.set($event.value)">@for (annotation of annotations.items(); track annotation.id) { <mat-option [value]="annotation.id">#{{ annotation.id }} / {{ annotation.annotator }}</mat-option> }</mat-select></mat-form-field></div>
        <div class="label-columns">
          <article><header><strong>{{ left()?.annotator ?? 'Left result' }}</strong>@if (left(); as item) { <app-annotation-state-badge [state]="item.annotation_state" /> }</header>@for (label of left()?.labels ?? []; track $index) { <div><span>{{ label.unit_key }}</span><strong>{{ label.label }}</strong><code>{{ interval(label) }}</code></div> } @empty { <p>No labels selected</p> }</article>
          <article><header><strong>{{ right()?.annotator ?? 'Right result' }}</strong>@if (right(); as item) { <app-annotation-state-badge [state]="item.annotation_state" /> }</header>@for (label of right()?.labels ?? []; track $index) { <div><span>{{ label.unit_key }}</span><strong>{{ label.label }}</strong><code>{{ interval(label) }}</code></div> } @empty { <p>No labels selected</p> }</article>
        </div>
        <app-diff-evidence-drawer [evidence]="evidence()" />
        @if (versionLinks().length > 1) {
          <section class="version-chain">
            <header><span>Version chain</span><small>{{ versionLinks().length }} revision(s) for {{ annotations.selected()?.annotator }} / {{ annotations.selected()?.item_key }}</small></header>
            <ol>
              @for (link of versionLinks(); track link.id) {
                <li [class.current]="link.id === annotations.selected()?.id">
                  <div class="chain-head"><button type="button" (click)="jumpToVersion(link.id)"><strong>#{{ link.id }}</strong></button><app-annotation-state-badge [state]="link.annotation_state" />@if (link.supersedes_id) { <code>replaces #{{ link.supersedes_id }}</code> }</div>
                  @if (link.replacement_reason) { <p class="chain-reason"><lucide-icon name="git-branch" [size]="13" />{{ link.replacement_reason }}</p> }
                  <small>{{ link.labels.length }} labels · updated {{ link.updated_at | date:'short' }}</small>
                </li>
              }
            </ol>
          </section>
        }
      </section>
    </section>
    @if (editorOpen()) {
      <section class="editor-band">
        <header><div><span>{{ editing() ? 'Draft revision' : 'New result set' }}</span><h2>{{ editing()?.item_key ?? 'Annotation payload' }}</h2></div><button mat-icon-button type="button" aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="18" /></button></header>
        <form [formGroup]="form" (ngSubmit)="save()">
          <mat-form-field appearance="outline"><mat-label>Dataset</mat-label><mat-select formControlName="dataset_id" (selectionChange)="selectEditorDataset($event.value)">@for (dataset of frozenDatasets(); track dataset.id) { <mat-option [value]="dataset.id">{{ dataset.dataset_code }}</mat-option> }</mat-select></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Published schema</mat-label><mat-select formControlName="schema_id">@for (schema of publishedSchemas(); track schema.id) { <mat-option [value]="schema.id">{{ schema.schema_code }} v{{ schema.version }}</mat-option> }</mat-select></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Item key</mat-label><input matInput formControlName="item_key" /></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Source SHA-256</mat-label><input matInput formControlName="source_checksum" /></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Labels (JSON)</mat-label><textarea matInput rows="9" formControlName="labels"></textarea></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Quality note</mat-label><textarea matInput rows="3" formControlName="quality_note"></textarea></mat-form-field>
          <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || annotations.loading()"><lucide-icon name="save" [size]="16" />{{ editing() ? 'Save draft' : 'Create draft' }}</button></div>
        </form>
      </section>
    }
    @if (replaceOpen()) {
      <section class="replace-band">
        <header><div><span>Tracked replacement</span><h2>New draft from result #{{ replaceTarget()?.id }}</h2></div><button mat-icon-button type="button" aria-label="Close replacement" (click)="closeReplace()"><lucide-icon name="x" [size]="18" /></button></header>
        <p class="replace-hint"><lucide-icon name="info" [size]="15" />The new draft keeps the same annotator and item. The old result leaves new comparisons immediately; adjudications not yet accepted become pending recomputation, while accepted history stays read-only.</p>
        <form [formGroup]="replaceForm" (ngSubmit)="submitReplace()">
          <mat-form-field appearance="outline" class="wide"><mat-label>Replacement reason</mat-label><textarea matInput rows="3" formControlName="reason" placeholder="Describe the missed label or incorrect boundary."></textarea></mat-form-field>
          <div class="form-actions"><button mat-button type="button" (click)="closeReplace()">Cancel</button><button mat-flat-button color="accent" type="submit" [disabled]="replaceForm.invalid || annotations.loading()"><lucide-icon name="copy" [size]="16" />Create replacement draft</button></div>
        </form>
      </section>
    }
  `,
  styles: [`
    .filter-band{display:grid;grid-template-columns:minmax(190px,.7fr) minmax(220px,1fr) auto;gap:10px;align-items:start;margin-bottom:16px;padding:11px;background:#e7ecea;border:1px solid #c3cdca;border-radius:4px}.filter-band button{height:40px;display:flex;gap:6px}.annotation-layout{display:grid;grid-template-columns:minmax(330px,.62fr) minmax(570px,1.38fr);gap:16px;align-items:start}.register,.compare-panel,.editor-band{background:#fbfcfa;border:1px solid #c1cbc8;border-radius:4px;overflow:hidden}.register>header{display:flex;justify-content:space-between;padding:11px 13px;background:#e7ecea;border-bottom:1px solid #c8d1ce}.register header span{font-size:11px;font-weight:750;text-transform:uppercase}.register header small{font-size:10px}.annotation-row{width:100%;display:grid;grid-template-columns:minmax(0,1fr) 30px auto;align-items:center;gap:9px;padding:11px;background:#fbfcfa;border:0;border-bottom:1px solid #dce2e0;text-align:left;cursor:pointer}.annotation-row:hover,.annotation-row.selected{background:#fff4cf}.annotation-row>span:first-child{display:grid;gap:3px;min-width:0}.annotation-row strong{font-size:11px}.annotation-row small{overflow:hidden;color:#6a767a;font-size:9px;text-overflow:ellipsis;white-space:nowrap}.label-count{height:27px;display:grid;place-items:center;color:#eef2f0;background:#435157;border-radius:3px;font-size:10px;font-weight:750}.selected-actions{display:grid;gap:8px;padding:11px;background:#eef2f0}.selected-actions>span{font-size:9px;text-transform:uppercase}.selected-actions>div{display:flex;gap:7px;flex-wrap:wrap}.compare-panel>header,.editor-band>header{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #cbd3d0}.compare-panel header span,.editor-band header span{color:#69767a;font-size:9px;text-transform:uppercase}.compare-panel h2,.editor-band h2{margin:3px 0 0;font-size:17px}.compare-panel>header>small{color:#34624d;font-size:9px;text-transform:uppercase}.compare-selectors{display:grid;grid-template-columns:repeat(2,1fr);gap:12px;padding:12px 14px 0}.label-columns{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:1px;margin:0 14px 14px;background:#cbd3d0;border:1px solid #cbd3d0}.label-columns>article{min-width:0;background:#f3f6f4}.label-columns article>header{display:flex;align-items:center;justify-content:space-between;gap:7px;padding:9px;background:#e4eae7}.label-columns article>header strong{font-size:10px}.label-columns article>div{display:grid;grid-template-columns:minmax(0,1fr) auto auto;gap:8px;padding:8px 9px;border-top:1px solid #d9dfdd;font-size:10px}.label-columns article>div span{overflow:hidden;text-overflow:ellipsis}.label-columns article>div strong{color:#6d4f0c}.label-columns code{font-size:9px}.label-columns p{margin:0;padding:18px;color:#6b777b;font-size:10px;text-align:center}.compare-panel app-diff-evidence-drawer{display:block;margin:14px}.editor-band{margin-top:16px}.editor-band form{display:grid;grid-template-columns:repeat(3,1fr);gap:4px 12px;padding:16px}.editor-band .wide,.form-actions{grid-column:1/-1}.form-actions{display:flex;justify-content:flex-end;gap:8px}
    .retired-note{display:flex;align-items:center;gap:6px;margin:8px 0 0;padding:7px 9px;color:#7a2f29;background:#f8ecea;border:1px solid #d8a59f;border-radius:3px;font-size:10px}.replace-reason{display:flex;align-items:flex-start;gap:6px;margin:8px 0 0;padding:7px 9px;color:#35594a;background:#eaf3ee;border:1px solid #a3c6b1;border-radius:3px;font-size:10px}.version-chain{margin:14px;border:1px solid #d3dad8;border-radius:4px}.version-chain>header{display:flex;justify-content:space-between;gap:8px;padding:9px 11px;background:#edf1ef;border-bottom:1px solid #d8dedc}.version-chain header span{font-size:10px;font-weight:750;text-transform:uppercase}.version-chain header small{font-size:9px;color:#69767a}.version-chain ol{list-style:none;margin:0;padding:0}.version-chain li{padding:10px 12px;border-bottom:1px solid #e2e7e5}.version-chain li:last-child{border-bottom:0}.version-chain li.current{background:#fff4cf}.chain-head{display:flex;align-items:center;gap:8px}.chain-head button{padding:0;border:0;background:none;cursor:pointer}.chain-head code{font-size:9px;color:#5a6c66}.chain-reason{display:flex;align-items:flex-start;gap:6px;margin:6px 0 3px;color:#3f5b4e;font-size:10px}.version-chain li>small{color:#6a767a;font-size:9px}.replace-band{margin-top:16px;background:#fbfcfa;border:1px solid #c1cbc8;border-radius:4px;overflow:hidden}.replace-band>header{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #cbd3d0;background:#eef3f0}.replace-band header span{color:#69767a;font-size:9px;text-transform:uppercase}.replace-band h2{margin:3px 0 0;font-size:17px}.replace-hint{display:flex;align-items:flex-start;gap:7px;margin:12px 15px 0;padding:9px 11px;color:#42564e;background:#e9f1eb;border-left:3px solid #458063;font-size:10px;line-height:1.45}.replace-band form{display:grid;grid-template-columns:1fr;gap:4px 12px;padding:14px 15px 16px}.replace-band .wide{grid-column:1/-1}
    @media(max-width:1100px){.annotation-layout{grid-template-columns:1fr}}@media(max-width:700px){.filter-band,.compare-selectors,.label-columns,.editor-band form{grid-template-columns:1fr}.editor-band .wide,.form-actions{grid-column:1}.annotation-row{grid-template-columns:minmax(0,1fr) 28px}.annotation-row app-annotation-state-badge{grid-column:1/-1}.label-columns{gap:12px;background:transparent;border:0}.label-columns>article{border:1px solid #cbd3d0}}
  `],
})
export class AnnotationsPage implements OnInit {
  private readonly fb = inject(FormBuilder);
  readonly datasets = inject(CorpusDatasetStore);
  readonly schemas = inject(AnnotationSchemaStore);
  readonly annotations = inject(AnnotationSetStore);
  readonly auth = useAuth();
  readonly leftId = signal(0);
  readonly rightId = signal(0);
  readonly editorOpen = signal(false);
  readonly editing = signal<AnnotationSet | null>(null);
  readonly editorError = signal('');
  readonly replaceOpen = signal(false);
  readonly replaceTarget = signal<AnnotationSet | null>(null);
  readonly replaceForm = this.fb.nonNullable.group({
    reason: ['', [Validators.required, Validators.minLength(4), Validators.maxLength(600)]],
  });
  readonly versionLinks = computed<AnnotationVersionLink[]>(() => this.annotations.versionChain()?.links ?? []);
  datasetFilter: number | null = null;
  itemFilter = 'DOC-7F2A';
  readonly frozenDatasets = computed(() => this.datasets.items().filter((dataset) => dataset.dataset_state === 'frozen'));
  readonly publishedSchemas = computed(() => this.schemas.items().filter((schema) => schema.schema_state === 'published' && (!this.form.controls.dataset_id.value || schema.dataset_id === this.form.controls.dataset_id.value)));
  readonly left = computed<AnnotationSet | null>(() => this.annotations.items().find((item) => item.id === this.leftId()) ?? this.annotations.items()[0] ?? null);
  readonly right = computed<AnnotationSet | null>(() => this.annotations.items().find((item) => item.id === this.rightId()) ?? this.annotations.items().find((item) => item.id !== this.left()?.id) ?? null);
  readonly comparisonKey = computed(() => this.left()?.item_key ?? this.right()?.item_key ?? 'Select two result sets');
  readonly evidence = computed(() => compareLabels(this.left()?.labels ?? [], this.right()?.labels ?? []));
  readonly form = this.fb.nonNullable.group({
    dataset_id: [0, [Validators.required, Validators.min(1)]], schema_id: [0, [Validators.required, Validators.min(1)]], item_key: ['DOC-QA-01', [Validators.required, Validators.minLength(2)]],
    source_checksum: ['aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', [Validators.required, Validators.minLength(64), Validators.maxLength(64)]],
    labels: ['[\n  {"unit_key":"document_class","label":"RISK"}\n]', Validators.required], quality_note: ['Checked against the published schema.'],
  });

  canCreate = () => this.auth.can('annotator', 'admin');
  canManage = () => this.auth.can('data_manager', 'adjudicator', 'admin');
  owns(annotation: AnnotationSet): boolean { return annotation.annotator_id === this.auth.user()?.id; }
  canEdit(annotation: AnnotationSet): boolean { return annotation.annotation_state === 'draft' && annotation.annotator_id === this.auth.user()?.id; }
  canReplace(annotation: AnnotationSet): boolean {
    return (this.owns(annotation) || this.auth.can('admin'))
      && (annotation.annotation_state === 'locked' || annotation.annotation_state === 'compared')
      && !annotation.superseded_by_id;
  }
  openReplace(annotation: AnnotationSet): void {
    this.replaceTarget.set(annotation);
    this.replaceForm.reset({ reason: '' });
    this.replaceOpen.set(true);
  }
  closeReplace(): void { this.replaceOpen.set(false); this.replaceTarget.set(null); }
  submitReplace(): void {
    const target = this.replaceTarget();
    if (!target || this.replaceForm.invalid) return;
    const reason = this.replaceForm.getRawValue().reason.trim();
    this.annotations.replace(target, reason, () => { this.closeReplace(); this.reload(); });
  }
  jumpToVersion(id: number): void {
    const match = this.annotations.items().find((item) => item.id === id);
    if (match) this.annotations.choose(match);
  }
  ngOnInit(): void { this.datasets.load(); this.schemas.load(); this.annotations.load(undefined, this.itemFilter); }
  reload(): void { this.datasets.load(); this.schemas.load(this.datasetFilter ?? undefined); this.annotations.load(this.datasetFilter ?? undefined, this.itemFilter); }
  applyFilters(): void { this.schemas.load(this.datasetFilter ?? undefined); this.annotations.load(this.datasetFilter ?? undefined, this.itemFilter.trim()); this.leftId.set(0); this.rightId.set(0); }
  openCreate(): void {
    const datasetID = this.datasetFilter ?? this.frozenDatasets()[0]?.id ?? 0;
    const schemaID = this.schemas.items().find((schema) => schema.dataset_id === datasetID && schema.schema_state === 'published')?.id ?? 0;
    this.schemas.load(datasetID || undefined); this.editing.set(null); this.editorError.set(''); this.form.enable();
    this.form.reset({ dataset_id: datasetID, schema_id: schemaID, item_key: 'DOC-QA-01', source_checksum: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', labels: '[\n  {"unit_key":"document_class","label":"RISK"}\n]', quality_note: 'Checked against the published schema.' });
    this.editorOpen.set(true);
  }
  selectEditorDataset(datasetID: number): void { this.form.controls.schema_id.setValue(0); this.schemas.load(datasetID); }
  openEdit(annotation: AnnotationSet): void {
    this.editing.set(annotation); this.editorError.set(''); this.schemas.load(annotation.dataset_id);
    this.form.setValue({ dataset_id: annotation.dataset_id, schema_id: annotation.schema_id, item_key: annotation.item_key, source_checksum: annotation.source_checksum, labels: JSON.stringify(annotation.labels, null, 2), quality_note: annotation.quality_note });
    this.form.controls.dataset_id.disable(); this.form.controls.schema_id.disable(); this.form.controls.item_key.disable(); this.form.controls.source_checksum.disable(); this.editorOpen.set(true);
  }
  closeEditor(): void { this.editorOpen.set(false); this.editing.set(null); this.editorError.set(''); this.form.enable(); }
  save(): void {
    if (this.form.invalid) return;
    const value = this.form.getRawValue();
    let labels: AnnotationLabel[];
    try { labels = JSON.parse(value.labels) as AnnotationLabel[]; } catch { this.editorError.set('Labels must be a valid JSON array.'); return; }
    if (!Array.isArray(labels) || labels.length === 0) { this.editorError.set('At least one label is required.'); return; }
    const current = this.editing();
    if (current) this.annotations.update(current.id, { labels, quality_note: value.quality_note }, () => this.closeEditor());
    else this.annotations.create({ dataset_id: value.dataset_id, schema_id: value.schema_id, item_key: value.item_key, source_checksum: value.source_checksum, labels, quality_note: value.quality_note }, () => this.closeEditor());
  }
  interval(label: AnnotationLabel): string { return label.end !== undefined && label.start !== undefined && label.end > label.start ? `[${label.start}, ${label.end})` : 'class'; }
}

function compareLabels(left: AnnotationLabel[], right: AnnotationLabel[]): DiffEvidence[] {
  const evidence: DiffEvidence[] = [];
  const matched = new Set<number>();
  for (const source of left) {
    const sourceSpan = isSpan(source);
    let index = right.findIndex((target, position) => !matched.has(position) && target.unit_key === source.unit_key && isSpan(target) === sourceSpan && (!sourceSpan || overlaps(source, target)));
    if (index < 0) {
      evidence.push({ type: 'omission', unit_key: source.unit_key, left_label: source.label, left_start: source.start, left_end: source.end, evidence: 'The right result does not contain a comparable rating or span.' });
      continue;
    }
    matched.add(index);
    const target = right[index];
    if (sourceSpan && (source.start !== target.start || source.end !== target.end)) {
      const type = source.label === target.label ? 'boundary' : 'overlap';
      evidence.push({ type, unit_key: source.unit_key, left_label: source.label, right_label: target.label, left_start: source.start, left_end: source.end, right_start: target.start, right_end: target.end, overlap: overlapSize(source, target), evidence: type === 'boundary' ? 'Labels agree but character boundaries differ.' : 'Overlapping spans carry different labels.' });
    } else if (source.label !== target.label) {
      evidence.push({ type: 'label', unit_key: source.unit_key, left_label: source.label, right_label: target.label, evidence: 'The same unit received different labels.' });
    }
  }
  right.forEach((target, index) => { if (!matched.has(index)) evidence.push({ type: 'omission', unit_key: target.unit_key, right_label: target.label, right_start: target.start, right_end: target.end, evidence: 'The left result does not contain a comparable rating or span.' }); });
  return evidence;
}

function isSpan(label: AnnotationLabel): boolean { return label.start !== undefined && label.end !== undefined && label.end > label.start; }
function overlaps(left: AnnotationLabel, right: AnnotationLabel): boolean { return Math.min(left.end ?? 0, right.end ?? 0) > Math.max(left.start ?? 0, right.start ?? 0); }
function overlapSize(left: AnnotationLabel, right: AnnotationLabel): number { return Math.max(0, Math.min(left.end ?? 0, right.end ?? 0) - Math.max(left.start ?? 0, right.start ?? 0)); }
