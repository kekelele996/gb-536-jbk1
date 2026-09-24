import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { LucideAngularModule } from 'lucide-angular';

@Component({
  selector: 'app-annotation-state-badge',
  standalone: true,
  imports: [LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <span class="state" [class]="tone()">
      <lucide-icon [name]="icon()" [size]="13" aria-hidden="true" />
      {{ label() }}
    </span>
  `,
  styles: [`
    .state{display:inline-flex;align-items:center;gap:5px;min-height:24px;padding:2px 8px;border:1px solid;border-radius:3px;font-size:11px;font-weight:700;line-height:1;white-space:nowrap;text-transform:capitalize}
    .good{color:#245f45;background:#edf6f0;border-color:#9fc6ad}.warn{color:#725313;background:#fff7dd;border-color:#d7bd6b}
    .bad{color:#842f2a;background:#f9ecea;border-color:#dba49e}.neutral{color:#566267;background:#eef1f1;border-color:#c5cdcf}
    .info{color:#2f5e72;background:#eaf3f6;border-color:#a6c5d1}
  `],
})
export class AnnotationStateBadgeComponent {
  readonly state = input.required<string>();
  readonly label = computed(() => this.state().replaceAll('_', ' '));
  readonly tone = computed(() => {
    if (['accepted', 'published', 'frozen'].includes(this.state())) return 'state good';
    if (['returned', 'open', 'reopened'].includes(this.state())) return 'state bad';
    if (['draft', 'assigned'].includes(this.state())) return 'state warn';
    if (['submitted', 'locked', 'compared', 'validated', 'adjudicated', 'reviewed'].includes(this.state())) return 'state info';
    return 'state neutral';
  });
  readonly icon = computed(() => {
    if (['accepted', 'published', 'frozen'].includes(this.state())) return 'circle-check';
    if (['returned', 'reopened'].includes(this.state())) return 'triangle-alert';
    if (['archived', 'deprecated', 'superseded'].includes(this.state())) return 'archive';
    if (['locked', 'reviewed'].includes(this.state())) return 'lock-keyhole';
    return 'circle-dot';
  });
}
