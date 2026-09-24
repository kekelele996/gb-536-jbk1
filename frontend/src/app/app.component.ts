import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { LucideAngularModule } from 'lucide-angular';
import { useAuth } from '../hooks/use-auth';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatButtonModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (auth.authenticated()) {
      <div class="app-frame">
        <header class="topbar">
          <a class="brand" routerLink="/datasets">
            <span class="brand-mark"><lucide-icon name="git-compare-arrows" [size]="21" /></span>
            <span><strong>Concordance / 536</strong><small>语料一致性裁决台</small></span>
          </a>
          <div class="session">
            <span><b>{{ auth.user()?.username }}</b><small>{{ roleLabel() }}</small></span>
            <button mat-icon-button type="button" aria-label="Sign out" title="Sign out" (click)="logout()"><lucide-icon name="log-out" [size]="18" /></button>
          </div>
        </header>
        <nav class="rail" aria-label="Primary navigation">
          <a routerLink="/datasets" routerLinkActive="active"><lucide-icon name="database" [size]="17" /><span>Datasets</span></a>
          <a routerLink="/schemas" routerLinkActive="active"><lucide-icon name="tags" [size]="17" /><span>Schemas</span></a>
          <a routerLink="/annotations" routerLinkActive="active"><lucide-icon name="scan-text" [size]="17" /><span>Compare</span></a>
          <a routerLink="/adjudication" routerLinkActive="active"><lucide-icon name="gavel" [size]="17" /><span>Decide</span></a>
          @if (auth.can('auditor', 'adjudicator', 'admin')) {
            <a routerLink="/audit" routerLinkActive="active"><lucide-icon name="clipboard-list" [size]="17" /><span>Audit</span></a>
          }
        </nav>
        <main class="workspace"><router-outlet /></main>
      </div>
    } @else {
      <router-outlet />
    }
  `,
  styles: [`
    .app-frame{min-height:100vh;display:grid;grid-template-columns:88px minmax(0,1fr);grid-template-rows:60px minmax(0,1fr);background:#f3f5f4}.topbar{position:sticky;top:0;z-index:20;grid-column:1/-1;display:flex;align-items:center;justify-content:space-between;padding:0 18px;color:#f6f8f7;background:#243238;border-bottom:3px solid #d7a728}.brand{display:flex;align-items:center;gap:10px;color:inherit;text-decoration:none}.brand-mark{width:35px;height:35px;display:grid;place-items:center;color:#243238;background:#e1b438;border-radius:3px}.brand span:last-child{display:grid}.brand strong{font-size:14px}.brand small{color:#b7c1c2;font-size:9px;text-transform:uppercase}.session{display:flex;align-items:center;gap:8px}.session>span{display:grid;text-align:right}.session b{font-size:11px}.session small{color:#b7c1c2;font-size:9px;text-transform:uppercase}.session button{color:#f6f8f7}.rail{position:sticky;top:60px;height:calc(100vh - 60px);display:flex;flex-direction:column;padding:12px 8px;gap:4px;background:#e2e7e5;border-right:1px solid #c2ccca}.rail a{height:55px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:5px;color:#536166;border-radius:3px;font-size:10px;text-decoration:none}.rail a:hover{color:#1f2d32;background:#edf1ef}.rail a.active{color:#1d292e;background:#fbfcfa;box-shadow:inset 3px 0 #c99717;font-weight:750}.workspace{min-width:0;padding:22px clamp(14px,2.4vw,34px) 42px}
    @media(max-width:760px){.app-frame{display:block;padding-top:60px}.topbar{position:fixed;left:0;right:0;height:60px}.session>span{display:none}.rail{position:fixed;z-index:18;top:auto;bottom:0;left:0;right:0;height:64px;flex-direction:row;justify-content:space-around;padding:5px 4px;border-top:1px solid #adb9b6;border-right:0}.rail a{flex:1;height:53px}.rail a.active{box-shadow:inset 0 3px #c99717}.workspace{padding:16px 12px 84px}}
  `],
})
export class AppComponent {
  readonly auth = useAuth();
  private readonly router = inject(Router);

  roleLabel(): string { return (this.auth.role() ?? '').replaceAll('_', ' '); }
  logout(): void { this.auth.logout(); void this.router.navigate(['/login']); }
}
