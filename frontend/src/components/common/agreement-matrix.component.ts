import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { ConfusionCell } from '../../types/adjudication-case';

@Component({
  selector: 'app-agreement-matrix',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="matrix" aria-label="Label confusion matrix">
      <header><span>Left annotation</span><strong>Label confusion</strong><span>Right annotation</span></header>
      @if (labels().length) {
        <div class="scroll">
          <table>
            <thead><tr><th scope="col">Left \ Right</th>@for (label of labels(); track label) { <th scope="col">{{ label }}</th> }</tr></thead>
            <tbody>
              @for (left of labels(); track left) {
                <tr><th scope="row">{{ left }}</th>@for (right of labels(); track right) {
                  <td [class.match]="left === right && value(left, right) > 0" [class.conflict]="left !== right && value(left, right) > 0">{{ value(left, right) }}</td>
                }</tr>
              }
            </tbody>
          </table>
        </div>
      } @else {
        <p>No confusion evidence recorded</p>
      }
    </section>
  `,
  styles: [`
    .matrix{min-width:0;border:1px solid #c4ccce;border-radius:4px;background:#f9faf8;overflow:hidden}.matrix header{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:9px 11px;background:#e7ebea;border-bottom:1px solid #c4ccce}.matrix header span{color:#687479;font-size:9px;text-transform:uppercase}.matrix header strong{font-size:11px;text-transform:uppercase}.scroll{max-width:100%;overflow:auto}table{width:100%;border-collapse:collapse;font-size:11px;font-variant-numeric:tabular-nums}th,td{min-width:74px;padding:9px;border-right:1px solid #d9dfdf;border-bottom:1px solid #d9dfdf;text-align:center}th{color:#49565b;background:#f0f3f2;font-size:9px;text-transform:uppercase}tbody th{text-align:left;position:sticky;left:0}td.match{color:#245f45;background:#eaf4ed;font-weight:800}td.conflict{color:#6e4e0b;background:#fff1bf;font-weight:800}.matrix p{margin:0;padding:20px;color:#6c787c;font-size:11px;text-align:center}
  `],
})
export class AgreementMatrixComponent {
  readonly cells = input<ConfusionCell[]>([]);
  readonly labels = computed(() => {
    const values = new Set<string>();
    for (const cell of this.cells()) {
      values.add(cell.left_label);
      values.add(cell.right_label);
    }
    return [...values].sort();
  });

  value(left: string, right: string): number {
    return this.cells().find((cell) => cell.left_label === left && cell.right_label === right)?.count ?? 0;
  }
}
