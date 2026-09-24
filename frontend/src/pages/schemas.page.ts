import { ChangeDetectionStrategy, Component, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { LucideAngularModule } from 'lucide-angular';
import { AnnotationStateBadgeComponent } from '../components/common/annotation-state-badge.component';
import { useAuth } from '../hooks/use-auth';
import { AnnotationSchemaStore } from '../stores/annotation-schema.store';
import { CorpusDatasetStore } from '../stores/corpus-dataset.store';
import { AnnotationSchema, LabelDefinition, MaskedExample } from '../types/annotation-schema';

@Component({
  standalone: true,
  imports: [DatePipe, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, LucideAngularModule, AnnotationStateBadgeComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head"><div><span>Controlled labeling contract</span><h1>Annotation schemas</h1></div><div class="head-actions"><button mat-stroked-button type="button" (click)="reload()" [disabled]="schemas.loading()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button>@if (canEdit()) { <button mat-flat-button color="primary" type="button" (click)="openCreate()" [disabled]="schemas.loading()"><lucide-icon name="plus" [size]="16" />New schema</button> }</div></section>
    @if (schemas.error() || editorError()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ editorError() || schemas.error() }}</p> }
    <section class="filter-band"><mat-form-field appearance="outline"><mat-label>Dataset</mat-label><mat-select [value]="datasetFilter()" (selectionChange)="filterByDataset($event.value)"><mat-option [value]="0">All datasets</mat-option>@for (dataset of datasets.items(); track dataset.id) { <mat-option [value]="dataset.id">{{ dataset.dataset_code }} / {{ dataset.name }}</mat-option> }</mat-select></mat-form-field><span>{{ schemas.items().length }} schema versions</span></section>
    <section class="schema-layout">
      <div class="schema-register">
        <header><span>Schema register</span><small>Version history is immutable after publication</small></header>
        @for (schema of schemas.items(); track schema.id) {
          <button type="button" class="schema-row" [class.selected]="schemas.selected()?.id === schema.id" (click)="schemas.choose(schema)">
            <span><strong>{{ schema.schema_code }} <i>v{{ schema.version }}</i></strong><small>{{ schema.dataset_code }} / {{ schema.label_definitions.length }} labels</small></span><app-annotation-state-badge [state]="schema.schema_state" />
          </button>
        } @empty { <p class="empty">No schema versions match this dataset</p> }
      </div>
      <aside class="schema-detail">
        @if (schemas.selected(); as schema) {
          <header><div><span>{{ schema.dataset_code }} / created {{ schema.created_at | date:'mediumDate' }}</span><h2>{{ schema.schema_code }} v{{ schema.version }}</h2></div><app-annotation-state-badge [state]="schema.schema_state" /></header>
          <section class="policy-grid"><article><span>Span policy</span><p>{{ schema.span_policy }}</p></article><article><span>Overlap policy</span><p>{{ schema.overlap_policy }}</p></article></section>
          <section class="definition-list"><header><span>Label definitions</span><small>{{ schema.label_definitions.length }} labels</small></header>@for (definition of schema.label_definitions; track definition.code) { <article><strong>{{ definition.code }}</strong><span>{{ definition.display_name }}</span><i>{{ definition.task_type }}</i><p>{{ definition.description }}</p></article> }</section>
          <section class="masked-examples"><span>Masked examples</span>@for (example of schema.examples; track example.item_key) { <article><strong>{{ example.item_key }}</strong><code>{{ example.masked_text }}</code></article> } @empty { <p>No examples supplied</p> }</section>
          @if (canEdit()) { <div class="detail-actions"><button mat-stroked-button type="button" (click)="openEdit(schema)" [disabled]="schema.schema_state !== 'draft' || schemas.loading()">Edit draft</button><button mat-stroked-button type="button" (click)="schemas.copy(schema, schema.version + 1)" [disabled]="schemas.loading()">Copy as v{{ schema.version + 1 }}</button>@if (schema.schema_state === 'draft') { <button mat-flat-button color="primary" type="button" (click)="schemas.transition(schema, 'validated')" [disabled]="schemas.loading()">Validate</button> } @if (schema.schema_state === 'validated') { <button mat-flat-button color="primary" type="button" (click)="schemas.transition(schema, 'published')" [disabled]="schemas.loading()">Publish</button> } @if (schema.schema_state === 'published') { <button mat-button class="danger" type="button" (click)="schemas.transition(schema, 'deprecated')" [disabled]="schemas.loading()">Deprecate</button> }</div> }
        } @else { <p class="empty">Select a schema version</p> }
      </aside>
    </section>
    @if (editorOpen()) {
      <section class="editor-band">
        <header><div><span>{{ editing() ? 'Draft revision' : 'Schema registration' }}</span><h2>{{ editing()?.schema_code ?? 'New annotation schema' }}</h2></div><button mat-icon-button type="button" aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="18" /></button></header>
        <form [formGroup]="form" (ngSubmit)="save()">
          <mat-form-field appearance="outline"><mat-label>Dataset</mat-label><mat-select formControlName="dataset_id">@for (dataset of datasets.items(); track dataset.id) { <mat-option [value]="dataset.id">{{ dataset.dataset_code }}</mat-option> }</mat-select></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Schema code</mat-label><input matInput formControlName="schema_code" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Version</mat-label><input matInput type="number" formControlName="version" /></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Label definitions (JSON)</mat-label><textarea matInput rows="8" formControlName="label_definitions"></textarea></mat-form-field>
          <mat-form-field appearance="outline" class="half"><mat-label>Span policy</mat-label><textarea matInput rows="3" formControlName="span_policy"></textarea></mat-form-field>
          <mat-form-field appearance="outline" class="half"><mat-label>Overlap policy</mat-label><textarea matInput rows="3" formControlName="overlap_policy"></textarea></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Masked examples (JSON)</mat-label><textarea matInput rows="5" formControlName="examples"></textarea></mat-form-field>
          <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || schemas.loading()"><lucide-icon name="save" [size]="16" />Save schema</button></div>
        </form>
      </section>
    }
  `,
  styles: [`
    .filter-band{display:flex;align-items:center;justify-content:space-between;gap:14px;margin-bottom:16px;padding:10px 12px;background:#e7ecea;border:1px solid #c4ceca;border-radius:4px}.filter-band mat-form-field{width:min(460px,75%);margin-bottom:-20px}.filter-band span{color:#647176;font-size:10px;text-transform:uppercase}.schema-layout{display:grid;grid-template-columns:minmax(320px,.62fr) minmax(560px,1.38fr);gap:16px;align-items:start}.schema-register,.schema-detail,.editor-band{background:#fbfcfa;border:1px solid #c1cbc8;border-radius:4px;overflow:hidden}.schema-register>header{display:flex;justify-content:space-between;gap:10px;padding:11px 13px;background:#e7ecea;border-bottom:1px solid #c8d1ce}.schema-register header span{font-size:11px;font-weight:750;text-transform:uppercase}.schema-register header small{color:#69767a;font-size:9px}.schema-row{width:100%;display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;gap:10px;padding:12px;background:#fbfcfa;border:0;border-bottom:1px solid #dce2e0;text-align:left;cursor:pointer}.schema-row:hover,.schema-row.selected{background:#fff4cf}.schema-row>span{display:grid;gap:4px;min-width:0}.schema-row strong{font-size:11px}.schema-row strong i{color:#6c777b;font-style:normal}.schema-row small{overflow:hidden;color:#69767a;font-size:9px;text-overflow:ellipsis;white-space:nowrap}.schema-detail>header,.editor-band>header{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #cad2cf}.schema-detail header span,.editor-band header span{color:#69767a;font-size:9px;text-transform:uppercase}.schema-detail h2,.editor-band h2{margin:3px 0 0;font-size:17px}.policy-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:1px;background:#cbd3d0;border-bottom:1px solid #cbd3d0}.policy-grid article{padding:12px;background:#eef2f0}.policy-grid span,.definition-list>header span,.masked-examples>span{color:#6b777b;font-size:9px;text-transform:uppercase}.policy-grid p{margin:5px 0 0;font-size:10px;line-height:1.5}.definition-list{padding:14px}.definition-list>header{display:flex;justify-content:space-between;margin-bottom:8px}.definition-list header small{font-size:9px}.definition-list article{display:grid;grid-template-columns:90px minmax(100px,.7fr) 90px minmax(160px,1.3fr);gap:8px;align-items:center;padding:8px;border-top:1px solid #dde3e0}.definition-list article strong{font-size:10px}.definition-list article span,.definition-list article p{font-size:10px}.definition-list article i{color:#795a11;font-size:9px;font-style:normal;text-transform:uppercase}.definition-list article p{margin:0;color:#59666a}.masked-examples{display:grid;gap:7px;margin:0 14px 14px;padding-top:12px;border-top:1px solid #d9dfdd}.masked-examples article{display:grid;grid-template-columns:110px minmax(0,1fr);gap:9px;padding:8px;background:#f0f3f1}.masked-examples strong,.masked-examples code{font-size:10px}.masked-examples code{overflow-wrap:anywhere}.masked-examples p{margin:0;font-size:10px}.detail-actions{display:flex;gap:8px;flex-wrap:wrap;padding:0 14px 14px}.danger{color:#95332d!important}.editor-band{margin-top:16px}.editor-band form{display:grid;grid-template-columns:repeat(2,1fr);gap:4px 12px;padding:16px}.editor-band form>mat-form-field:nth-child(-n+3){grid-column:auto}.editor-band .wide,.form-actions{grid-column:1/-1}.form-actions{display:flex;justify-content:flex-end;gap:8px}
    @media(max-width:1050px){.schema-layout{grid-template-columns:1fr}.definition-list article{grid-template-columns:80px 1fr 80px}.definition-list article p{grid-column:1/-1}}@media(max-width:650px){.filter-band{align-items:stretch;flex-direction:column}.filter-band mat-form-field{width:100%;margin:0}.editor-band form{grid-template-columns:1fr}.editor-band .wide,.form-actions{grid-column:1}.policy-grid{grid-template-columns:1fr}.definition-list article{grid-template-columns:1fr}.definition-list article p{grid-column:1}.masked-examples article{grid-template-columns:1fr}}
  `],
})
export class SchemasPage implements OnInit {
  private readonly fb = inject(FormBuilder);
  readonly datasets = inject(CorpusDatasetStore);
  readonly schemas = inject(AnnotationSchemaStore);
  readonly auth = useAuth();
  readonly datasetFilter = signal(0);
  readonly editorOpen = signal(false);
  readonly editing = signal<AnnotationSchema | null>(null);
  readonly editorError = signal('');
  readonly form = this.fb.nonNullable.group({
    dataset_id: [0, [Validators.required, Validators.min(1)]], schema_code: ['QUALITY-LABELS', [Validators.required, Validators.minLength(3)]], version: [1, [Validators.required, Validators.min(1)]],
    label_definitions: ['[\n  {"code":"RISK","display_name":"Risk","task_type":"classification","description":"Unit contains material risk."},\n  {"code":"CLEAR","display_name":"Clear","task_type":"classification","description":"Unit contains no material risk."}\n]', Validators.required],
    span_policy: ['Use half-open Unicode code-point offsets and exclude punctuation.', [Validators.required, Validators.minLength(8)]],
    overlap_policy: ['Nested spans are forbidden; adjacent spans are allowed.', [Validators.required, Validators.minLength(8)]], examples: ['[]', Validators.required],
  });

  canEdit = () => this.auth.can('data_manager', 'admin');
  ngOnInit(): void { this.datasets.load(); this.schemas.load(); }
  reload(): void { this.datasets.load(); this.schemas.load(this.datasetFilter() || undefined); }
  filterByDataset(id: number): void { this.datasetFilter.set(id); this.schemas.load(id || undefined); }
  openCreate(): void {
    const datasetID = this.datasetFilter() || this.datasets.items()[0]?.id || 0;
    this.editing.set(null); this.editorError.set(''); this.form.enable();
    this.form.reset({ dataset_id: datasetID, schema_code: 'QUALITY-LABELS', version: 1, label_definitions: '[\n  {"code":"RISK","display_name":"Risk","task_type":"classification","description":"Unit contains material risk."},\n  {"code":"CLEAR","display_name":"Clear","task_type":"classification","description":"Unit contains no material risk."}\n]', span_policy: 'Use half-open Unicode code-point offsets and exclude punctuation.', overlap_policy: 'Nested spans are forbidden; adjacent spans are allowed.', examples: '[]' });
    this.editorOpen.set(true);
  }
  openEdit(schema: AnnotationSchema): void {
    this.editing.set(schema); this.editorError.set('');
    this.form.setValue({ dataset_id: schema.dataset_id, schema_code: schema.schema_code, version: schema.version, label_definitions: JSON.stringify(schema.label_definitions, null, 2), span_policy: schema.span_policy, overlap_policy: schema.overlap_policy, examples: JSON.stringify(schema.examples, null, 2) });
    this.form.controls.dataset_id.disable(); this.form.controls.schema_code.disable(); this.form.controls.version.disable(); this.editorOpen.set(true);
  }
  closeEditor(): void { this.editorOpen.set(false); this.editing.set(null); this.editorError.set(''); this.form.enable(); }
  save(): void {
    if (this.form.invalid) return;
    const value = this.form.getRawValue();
    let definitions: LabelDefinition[];
    let examples: MaskedExample[];
    try { definitions = JSON.parse(value.label_definitions) as LabelDefinition[]; examples = JSON.parse(value.examples) as MaskedExample[]; }
    catch { this.editorError.set('Label definitions and examples must be valid JSON arrays.'); return; }
    if (!Array.isArray(definitions) || !Array.isArray(examples)) { this.editorError.set('Label definitions and examples must be JSON arrays.'); return; }
    const current = this.editing();
    const mutable = { version: value.version, label_definitions: definitions, span_policy: value.span_policy, overlap_policy: value.overlap_policy, examples };
    if (current) this.schemas.update(current.id, mutable, () => this.closeEditor());
    else this.schemas.create({ dataset_id: value.dataset_id, schema_code: value.schema_code, ...mutable }, () => this.closeEditor());
  }
}
