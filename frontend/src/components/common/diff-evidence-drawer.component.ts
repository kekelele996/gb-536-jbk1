import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatExpansionModule } from '@angular/material/expansion';
import { LucideAngularModule } from 'lucide-angular';
import { DiffEvidence } from '../../types/adjudication-case';

@Component({
  selector: 'app-diff-evidence-drawer',
  standalone: true,
  imports: [MatExpansionModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <mat-expansion-panel class="drawer" [expanded]="expanded()">
      <mat-expansion-panel-header>
        <mat-panel-title><lucide-icon name="scan-text" [size]="16" /> Difference evidence</mat-panel-title>
        <mat-panel-description>{{ evidence().length }} findings</mat-panel-description>
      </mat-expansion-panel-header>
      @for (item of evidence(); track $index) {
        <article class="finding">
          <header><span class="type"><lucide-icon [name]="icon(item.type)" [size]="14" />{{ item.type }}</span><strong>{{ item.unit_key }}</strong></header>
          <div class="labels"><span>{{ item.left_label || 'omitted' }} {{ interval(item.left_start, item.left_end) }}</span><i>versus</i><span>{{ item.right_label || 'omitted' }} {{ interval(item.right_start, item.right_end) }}</span></div>
          <p>{{ item.evidence }}</p>
        </article>
      } @empty {
        <p class="empty"><lucide-icon name="circle-check" [size]="15" /> No structural disagreement</p>
      }
    </mat-expansion-panel>
  `,
  styles: [`
    .drawer{border:1px solid #c4ccce;border-radius:4px!important;box-shadow:none!important}.mat-expansion-panel-header-title{display:flex;align-items:center;gap:7px;font-size:12px;font-weight:750}.mat-expansion-panel-header-description{justify-content:flex-end;font-size:10px}.finding{padding:10px 0;border-top:1px solid #dce1e0}.finding:first-of-type{border-top:0}.finding header{display:flex;align-items:center;justify-content:space-between;gap:9px}.finding header strong{font-size:11px}.type{display:inline-flex;align-items:center;gap:5px;color:#6f5010;font-size:10px;font-weight:750;text-transform:uppercase}.labels{display:grid;grid-template-columns:minmax(0,1fr) auto minmax(0,1fr);gap:7px;margin:8px 0}.labels span{min-width:0;padding:6px 8px;background:#f0f2f1;border:1px solid #d4dad8;font-size:10px;overflow-wrap:anywhere}.labels i{align-self:center;color:#7b8588;font-size:9px;font-style:normal;text-transform:uppercase}.finding p{margin:0;color:#556166;font-size:10px;line-height:1.5}.empty{display:flex;align-items:center;gap:6px;margin:0;color:#2f674b;font-size:11px}
    @media(max-width:520px){.labels{grid-template-columns:1fr}.labels i{text-align:center}}
  `],
})
export class DiffEvidenceDrawerComponent {
  readonly evidence = input<DiffEvidence[]>([]);
  readonly expanded = input(true);

  interval(start?: number, end?: number): string {
    return end !== undefined && start !== undefined && end > start ? `[${start}, ${end})` : '';
  }

  icon(type: string): string {
    if (type === 'omission') return 'circle-off';
    if (type === 'boundary') return 'brackets';
    if (type === 'overlap') return 'layers-3';
    return 'tag';
  }
}
