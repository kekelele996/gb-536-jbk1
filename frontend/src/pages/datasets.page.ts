import { ChangeDetectionStrategy, Component, computed, effect, inject, OnInit, signal } from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { LucideAngularModule } from 'lucide-angular';
import { AnnotationSchemaApi } from '../api/annotation-schema';
import { AdjudicationCaseApi } from '../api/adjudication-case';
import { AgreementMatrixComponent } from '../components/common/agreement-matrix.component';
import { AnnotationStateBadgeComponent } from '../components/common/annotation-state-badge.component';
import { useAuth } from '../hooks/use-auth';
import { CorpusDatasetStore } from '../stores/corpus-dataset.store';
import { AdjudicationCase } from '../types/adjudication-case';
import { AnnotationSchema } from '../types/annotation-schema';
import { CorpusDataset } from '../types/corpus-dataset';
import { bindDatasetDetails } from '../utils/dataset-detail-loader';

@Component({
  standalone: true,
  imports: [DatePipe, DecimalPipe, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, LucideAngularModule, AgreementMatrixComponent, AnnotationStateBadgeComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head">
      <div><span>Versioned corpus inventory</span><h1>Datasets</h1></div>
      <div class="head-actions"><button mat-stroked-button type="button" (click)="reload()" [disabled]="store.loading()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button>@if (canEdit()) { <button mat-flat-button color="primary" type="button" (click)="openCreate()" [disabled]="store.loading()"><lucide-icon name="plus" [size]="16" />New dataset</button> }</div>
    </section>
    <aside class="privacy-note"><lucide-icon name="shield-check" [size]="17" /><div><strong>Evidence boundary</strong><span>Only masked examples, checksums, and structural labels are available in this workspace.</span></div></aside>
    @if (store.error()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ store.error() }}</p> }
    <section class="dataset-layout">
      <div class="register">
        <header><span>{{ store.items().length }} datasets</span><small>Language / documents / state</small></header>
        <div class="table-scroll"><table><thead><tr><th>Dataset</th><th>Scope</th><th>Documents</th><th>Version</th><th>State</th></tr></thead><tbody>
          @for (dataset of store.items(); track dataset.id) {
            <tr [class.selected]="store.selected()?.id === dataset.id" (click)="store.choose(dataset)">
              <td><strong>{{ dataset.dataset_code }}</strong><small>{{ dataset.name }}</small></td>
              <td>{{ dataset.language }}<small>{{ dataset.domain }}</small></td>
              <td>{{ dataset.document_count | number }}</td><td>v{{ dataset.version }}</td><td><app-annotation-state-badge [state]="dataset.dataset_state" /></td>
            </tr>
          } @empty { <tr><td colspan="5" class="empty">No datasets available</td></tr> }
        </tbody></table></div>
      </div>
      <aside class="detail">
        @if (store.selected(); as dataset) {
          <header><div><span>{{ dataset.dataset_code }} / v{{ dataset.version }}</span><h2>{{ dataset.name }}</h2></div><app-annotation-state-badge [state]="dataset.dataset_state" /></header>
          <dl><div><dt>Owner</dt><dd>{{ dataset.owner_team }}</dd></div><div><dt>Domain</dt><dd>{{ dataset.domain }}</dd></div><div><dt>Schemas</dt><dd>{{ dataset.schema_count }}</dd></div><div><dt>Annotations</dt><dd>{{ dataset.annotation_count }}</dd></div><div class="wide"><dt>Mask policy</dt><dd>{{ dataset.content_mask_policy }}</dd></div><div class="wide"><dt>Updated</dt><dd>{{ dataset.updated_at | date:'medium' }}</dd></div></dl>
          <section class="schema-strip"><span>Schema versions</span>@for (schema of schemas(); track schema.id) { <article><strong>{{ schema.schema_code }} v{{ schema.version }}</strong><app-annotation-state-badge [state]="schema.schema_state" /></article> } @empty { <p>No schema registered</p> }</section>
          @if (latestCase(); as adjudication) { <app-agreement-matrix [cells]="adjudication.confusion_snapshot" /> }
          @if (canEdit()) { <div class="detail-actions"><button mat-stroked-button type="button" (click)="openEdit(dataset)" [disabled]="dataset.dataset_state !== 'draft' || store.loading()">Edit metadata</button>@if (dataset.dataset_state === 'draft') { <button mat-flat-button color="primary" type="button" (click)="store.transition(dataset, 'frozen')" [disabled]="store.loading()">Freeze version</button> } @if (dataset.dataset_state === 'frozen') { <button mat-button class="danger" type="button" (click)="store.transition(dataset, 'archived')" [disabled]="store.loading()">Archive</button> }</div> }
        } @else { <p class="empty">Select a dataset</p> }
      </aside>
    </section>
    @if (editorOpen()) {
      <section class="editor-band">
        <header><div><span>{{ editing() ? 'Metadata revision' : 'Dataset registration' }}</span><h2>{{ editing()?.dataset_code ?? 'New corpus dataset' }}</h2></div><button mat-icon-button type="button" aria-label="Close editor" (click)="closeEditor()"><lucide-icon name="x" [size]="18" /></button></header>
        <form [formGroup]="form" (ngSubmit)="save()">
          <mat-form-field appearance="outline"><mat-label>Dataset code</mat-label><input matInput formControlName="dataset_code" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Name</mat-label><input matInput formControlName="name" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Language</mat-label><input matInput formControlName="language" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Domain</mat-label><input matInput formControlName="domain" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Document count</mat-label><input matInput type="number" formControlName="document_count" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Owner team</mat-label><input matInput formControlName="owner_team" /></mat-form-field>
          <mat-form-field appearance="outline" class="wide"><mat-label>Content masking policy</mat-label><textarea matInput rows="3" formControlName="content_mask_policy"></textarea></mat-form-field>
          <div class="form-actions"><button mat-button type="button" (click)="closeEditor()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || store.loading()"><lucide-icon name="save" [size]="16" />Save dataset</button></div>
        </form>
      </section>
    }
  `,
  styles: [`
    .privacy-note{display:flex;align-items:flex-start;gap:9px;margin:0 0 16px;padding:10px 12px;color:#4d5c61;background:#e9f0ed;border:1px solid #b8cbc3;border-radius:3px}.privacy-note div{display:grid;gap:2px}.privacy-note strong{font-size:10px;text-transform:uppercase}.privacy-note span{font-size:11px}.dataset-layout{display:grid;grid-template-columns:minmax(560px,1.2fr) minmax(350px,.8fr);gap:16px;align-items:start}.register,.detail,.editor-band{background:#fbfcfa;border:1px solid #c1cbc8;border-radius:4px;overflow:hidden}.register>header{display:flex;justify-content:space-between;padding:10px 13px;background:#e7ecea;border-bottom:1px solid #c8d1ce}.register header span{font-size:11px;font-weight:750;text-transform:uppercase}.register header small{color:#667479;font-size:10px}.table-scroll{overflow:auto}table{width:100%;min-width:690px;border-collapse:collapse}th{padding:10px 12px;color:#6c787c;background:#f2f5f3;font-size:9px;text-align:left;text-transform:uppercase}td{padding:11px 12px;border-top:1px solid #e0e5e3;font-size:11px}td strong,td small{display:block}td small{margin-top:3px;color:#6a767a;font-size:9px}tbody tr{cursor:pointer}tbody tr:hover,tbody tr.selected{background:#fff4cf}.detail{position:sticky;top:82px}.detail>header,.editor-band>header{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #cbd3d0}.detail header span,.editor-band header span{color:#69767a;font-size:9px;text-transform:uppercase}.detail h2,.editor-band h2{margin:3px 0 0;font-size:17px}.detail dl{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin:14px}.detail dl .wide{grid-column:1/-1}.detail dt,.schema-strip>span{color:#718084;font-size:9px;text-transform:uppercase}.detail dd{margin:3px 0 0;font-size:11px;line-height:1.45}.schema-strip{display:grid;gap:6px;margin:14px;padding-top:12px;border-top:1px solid #d9dfdd}.schema-strip article{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:7px 8px;background:#eff2f1}.schema-strip article strong{font-size:10px}.schema-strip p{margin:0;color:#6b777b;font-size:10px}.detail app-agreement-matrix{display:block;margin:14px}.detail-actions{display:flex;gap:8px;flex-wrap:wrap;padding:0 14px 14px}.danger{color:#96342d!important}.editor-band{margin-top:16px}.editor-band form{display:grid;grid-template-columns:repeat(3,1fr);gap:4px 12px;padding:16px}.editor-band .wide,.form-actions{grid-column:1/-1}.form-actions{display:flex;justify-content:flex-end;gap:8px}
    @media(max-width:1100px){.dataset-layout{grid-template-columns:1fr}.detail{position:static}.editor-band form{grid-template-columns:repeat(2,1fr)}}@media(max-width:650px){.editor-band form{grid-template-columns:1fr}.editor-band .wide,.form-actions{grid-column:1}.register header small{display:none}}
  `],
})
export class DatasetsPage implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly schemaApi = inject(AnnotationSchemaApi);
  private readonly caseApi = inject(AdjudicationCaseApi);
  readonly store = inject(CorpusDatasetStore);
  readonly auth = useAuth();
  readonly schemas = signal<AnnotationSchema[]>([]);
  readonly cases = signal<AdjudicationCase[]>([]);
  readonly latestCase = computed(() => this.cases()[0] ?? null);
  readonly editorOpen = signal(false);
  readonly editing = signal<CorpusDataset | null>(null);
  readonly form = this.fb.nonNullable.group({
    dataset_code: ['NLP-QA-ZH', [Validators.required, Validators.minLength(3)]],
    name: ['Masked quality corpus', [Validators.required, Validators.minLength(3)]],
    language: ['zh-CN', [Validators.required, Validators.minLength(2)]],
    domain: ['quality-review', [Validators.required, Validators.minLength(2)]],
    document_count: [0, [Validators.required, Validators.min(0)]],
    content_mask_policy: ['Replace names and identifiers with [MASK] before review.', [Validators.required, Validators.minLength(8)]],
    owner_team: ['NLP Quality Lab', [Validators.required, Validators.minLength(2)]],
  });

  constructor() {
    effect((onCleanup) => {
      const dataset = this.store.selected();
      onCleanup(bindDatasetDetails(
        dataset?.id,
        (id) => this.schemaApi.list(id),
        (id) => this.caseApi.list(id),
        (items) => this.schemas.set(items),
        (items) => this.cases.set(items),
      ));
    }, { allowSignalWrites: true });
  }

  canEdit = () => this.auth.can('data_manager', 'admin');
  ngOnInit(): void { this.reload(); }
  reload(): void { this.store.load(); }
  openCreate(): void {
    this.editing.set(null); this.form.controls.dataset_code.enable();
    this.form.reset({ dataset_code: 'NLP-QA-ZH', name: 'Masked quality corpus', language: 'zh-CN', domain: 'quality-review', document_count: 0, content_mask_policy: 'Replace names and identifiers with [MASK] before review.', owner_team: 'NLP Quality Lab' });
    this.editorOpen.set(true);
  }
  openEdit(dataset: CorpusDataset): void {
    this.editing.set(dataset);
    this.form.setValue({ dataset_code: dataset.dataset_code, name: dataset.name, language: dataset.language, domain: dataset.domain, document_count: dataset.document_count, content_mask_policy: dataset.content_mask_policy, owner_team: dataset.owner_team });
    this.form.controls.dataset_code.disable(); this.editorOpen.set(true);
  }
  closeEditor(): void { this.editorOpen.set(false); this.editing.set(null); this.form.controls.dataset_code.enable(); }
  save(): void {
    if (this.form.invalid) return;
    const value = this.form.getRawValue();
    const current = this.editing();
    if (current) this.store.update(current.id, { name: value.name, language: value.language, domain: value.domain, document_count: value.document_count, content_mask_policy: value.content_mask_policy, owner_team: value.owner_team, version: current.version }, () => this.closeEditor());
    else this.store.create(value, () => this.closeEditor());
  }
}
