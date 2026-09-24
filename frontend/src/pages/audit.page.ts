import { ChangeDetectionStrategy, Component, computed, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { LucideAngularModule } from 'lucide-angular';
import { finalize } from 'rxjs';
import { AuditApi } from '../api/audit';
import { DiffEvidenceDrawerComponent } from '../components/common/diff-evidence-drawer.component';
import { AdjudicationCaseStore } from '../stores/adjudication-case.store';
import { AuditEvent } from '../types/adjudication-case';
import { apiErrorMessage } from '../utils/api-error';

@Component({
  standalone: true,
  imports: [DatePipe, FormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, LucideAngularModule, DiffEvidenceDrawerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="page-head"><div><span>Redacted immutable projections</span><h1>Audit center</h1></div><div class="head-actions"><button mat-stroked-button type="button" (click)="load()" [disabled]="loading()"><lucide-icon name="refresh-cw" [size]="16" />Refresh</button></div></section>
    <section class="audit-filters">
      <mat-form-field appearance="outline"><mat-label>Actor</mat-label><input matInput [(ngModel)]="actor" /></mat-form-field>
      <mat-form-field appearance="outline"><mat-label>Request ID</mat-label><input matInput [(ngModel)]="requestId" /></mat-form-field>
      <mat-form-field appearance="outline"><mat-label>Entity</mat-label><mat-select [(ngModel)]="resourceType"><mat-option value="">All entities</mat-option><mat-option value="corpus_dataset">Corpus dataset</mat-option><mat-option value="annotation_schema">Annotation schema</mat-option><mat-option value="annotation_set">Annotation set</mat-option><mat-option value="adjudication_case">Adjudication case</mat-option></mat-select></mat-form-field>
      <mat-form-field appearance="outline"><mat-label>Action</mat-label><input matInput [(ngModel)]="action" /></mat-form-field>
      <button mat-flat-button color="primary" type="button" (click)="load()" [disabled]="loading()"><lucide-icon name="filter" [size]="15" />Apply</button>
    </section>
    @if (error()) { <p class="error-banner"><lucide-icon name="triangle-alert" [size]="15" />{{ error() }}</p> }
    <section class="audit-layout">
      <div class="event-stream">
        <header><span>Append-only stream</span><small>{{ events().length }} events</small></header>
        @for (event of events(); track event.id) {
          <button type="button" class="event-row" [class.selected]="selected()?.id === event.id" (click)="selected.set(event)">
            <span class="event-icon"><lucide-icon [name]="icon(event.resource_type)" [size]="16" /></span><span class="event-main"><strong>{{ event.action }}</strong><small>{{ event.resource_type }} #{{ event.resource_id }} / {{ event.actor }}</small></span><span class="event-time">{{ event.created_at | date:'MMM d' }}<small>{{ event.created_at | date:'HH:mm:ss' }}</small></span>
          </button>
        } @empty { <p class="empty">No events match these filters</p> }
      </div>
      <aside class="audit-detail">
        @if (selected(); as event) {
          <header><div><span>Event #{{ event.id }}</span><h2>{{ event.action }}</h2></div><strong>{{ roleName(event.role) }}</strong></header>
          <dl><div><dt>Actor</dt><dd>{{ event.actor }}</dd></div><div><dt>Entity</dt><dd>{{ event.resource_type }} #{{ event.resource_id }}</dd></div><div class="wide"><dt>Request ID</dt><dd><code>{{ event.request_id }}</code></dd></div><div class="wide"><dt>Timestamp</dt><dd>{{ event.created_at | date:'medium' }} UTC</dd></div></dl>
          @if (caseEvidence().length) { <app-diff-evidence-drawer [evidence]="caseEvidence()" [expanded]="false" /> }
          <section class="redaction-note"><lucide-icon name="shield-check" [size]="15" /><span>Label payloads, masked examples, and source text fields are recursively redacted before persistence.</span></section>
          <section class="json-evidence"><div><span>Before</span><pre>{{ pretty(event.before) }}</pre></div><div><span>After</span><pre>{{ pretty(event.after) }}</pre></div><div><span>Parameters</span><pre>{{ pretty(event.parameters) }}</pre></div></section>
        } @else { <p class="empty">Select an event to inspect its immutable projection</p> }
      </aside>
    </section>
  `,
  styles: [`
    .audit-filters{display:grid;grid-template-columns:.75fr 1.2fr .9fr .9fr auto;gap:9px;align-items:start;margin-bottom:16px;padding:11px;background:#e7ecea;border:1px solid #c3cdca;border-radius:4px}.audit-filters button{height:40px;display:flex;gap:6px}.audit-layout{display:grid;grid-template-columns:minmax(360px,.68fr) minmax(540px,1.32fr);gap:16px;align-items:start}.event-stream,.audit-detail{background:#fbfcfa;border:1px solid #c1cbc8;border-radius:4px;overflow:hidden}.event-stream>header{display:flex;justify-content:space-between;padding:11px 13px;background:#e7ecea;border-bottom:1px solid #c8d1ce}.event-stream header span{font-size:11px;font-weight:750;text-transform:uppercase}.event-stream header small{font-size:10px}.event-row{width:100%;display:grid;grid-template-columns:34px minmax(0,1fr) auto;align-items:center;gap:9px;padding:11px;background:#fbfcfa;border:0;border-bottom:1px solid #dce2e0;text-align:left;cursor:pointer}.event-row:hover,.event-row.selected{background:#fff4cf}.event-icon{width:31px;height:31px;display:grid;place-items:center;color:#f3f6f4;background:#435157;border-radius:3px}.event-main{display:grid;gap:3px;min-width:0}.event-main strong{overflow:hidden;font-size:11px;text-overflow:ellipsis}.event-main small{overflow:hidden;color:#6a777a;font-size:9px;text-overflow:ellipsis;white-space:nowrap}.event-time{display:grid;gap:2px;text-align:right;font-size:9px;text-transform:uppercase}.event-time small{color:#6a777a}.audit-detail>header{display:flex;justify-content:space-between;gap:10px;padding:14px 15px;border-bottom:1px solid #cbd3d0}.audit-detail header span{color:#69767a;font-size:9px;text-transform:uppercase}.audit-detail h2{margin:3px 0 0;font-size:17px}.audit-detail header>strong{font-size:10px;text-transform:uppercase}.audit-detail dl{display:grid;grid-template-columns:repeat(2,1fr);gap:12px;margin:14px}.audit-detail dl .wide{grid-column:1/-1}.audit-detail dt{color:#718084;font-size:9px;text-transform:uppercase}.audit-detail dd{margin:3px 0;font-size:11px}.audit-detail code{word-break:break-all}.audit-detail app-diff-evidence-drawer{display:block;margin:14px}.redaction-note{display:flex;align-items:flex-start;gap:7px;margin:14px;padding:9px;color:#315d49;background:#edf4f0;border-left:3px solid #458063;font-size:10px;line-height:1.45}.json-evidence{display:grid;grid-template-columns:repeat(3,1fr);gap:1px;margin-top:14px;background:#c8d0cd;border-top:1px solid #c8d0cd}.json-evidence>div{min-width:0;padding:11px;background:#eef2f0}.json-evidence span{font-size:9px;text-transform:uppercase}.json-evidence pre{min-height:92px;max-height:230px;overflow:auto;margin:6px 0 0;color:#344146;font-size:9px;white-space:pre-wrap;word-break:break-word}
    @media(max-width:1120px){.audit-layout{grid-template-columns:1fr}.audit-filters{grid-template-columns:repeat(2,1fr)}.audit-filters button{grid-column:2}}@media(max-width:650px){.audit-filters,.json-evidence{grid-template-columns:1fr}.audit-filters button{grid-column:1}.event-row{grid-template-columns:32px minmax(0,1fr)}.event-time{grid-column:2;text-align:left}}
  `],
})
export class AuditPage implements OnInit {
  private readonly api = inject(AuditApi);
  private readonly cases = inject(AdjudicationCaseStore);
  readonly events = signal<AuditEvent[]>([]);
  readonly selected = signal<AuditEvent | null>(null);
  readonly error = signal('');
  readonly loading = signal(false);
  readonly caseEvidence = computed(() => {
    const event = this.selected();
    if (!event || event.resource_type !== 'adjudication_case') return [];
    return this.cases.items().find((item) => item.id === Number(event.resource_id))?.evidence_snapshot ?? [];
  });
  actor = '';
  requestId = '';
  resourceType = '';
  action = '';

  ngOnInit(): void { this.cases.load(); this.load(); }
  load(): void {
	this.loading.set(true);
    this.error.set('');
	this.api.list({ actor: this.actor.trim(), request_id: this.requestId.trim(), resource_type: this.resourceType, action: this.action.trim() }).pipe(finalize(() => this.loading.set(false))).subscribe({
      next: ({ data }) => { this.events.set(data); this.selected.set(data.find((event) => event.id === this.selected()?.id) ?? data[0] ?? null); },
      error: (error) => this.error.set(apiErrorMessage(error)),
    });
  }
  pretty(value: Record<string, unknown>): string { return JSON.stringify(value, null, 2); }
  roleName(role: string): string { return role.replaceAll('_', ' '); }
  icon(resource: string): string {
    if (resource === 'corpus_dataset') return 'database';
    if (resource === 'annotation_schema') return 'tags';
    if (resource === 'annotation_set') return 'scan-text';
    return 'gavel';
  }
}
