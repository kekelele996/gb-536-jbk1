import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { LucideAngularModule } from 'lucide-angular';
import { useAuth } from '../hooks/use-auth';
import { apiErrorMessage } from '../utils/api-error';

@Component({
  standalone: true,
  imports: [ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule, LucideAngularModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <main class="login-view">
      <section class="identity">
        <span class="emblem"><lucide-icon name="git-compare-arrows" [size]="34" /></span>
        <div><p>Internal NLP quality workspace</p><h1>Concordance</h1><span>语料一致性裁决台 / GB-536</span></div>
      </section>
      <section class="login-panel">
        <header><span>Authorized access</span><strong>Sign in to the workbench</strong></header>
        <div class="role-strip" aria-label="Demo profiles">
          @for (profile of profiles; track profile.username) {
            <button type="button" [class.active]="form.controls.username.value === profile.username" (click)="select(profile.username, profile.password)">{{ profile.label }}</button>
          }
        </div>
        <form [formGroup]="form" (ngSubmit)="submit()">
          <mat-form-field appearance="outline"><mat-label>Username</mat-label><input matInput formControlName="username" autocomplete="username" /></mat-form-field>
          <mat-form-field appearance="outline"><mat-label>Password</mat-label><input matInput type="password" formControlName="password" autocomplete="current-password" /></mat-form-field>
          @if (error()) { <p class="error"><lucide-icon name="triangle-alert" [size]="15" />{{ error() }}</p> }
          <button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || busy()"><lucide-icon name="log-in" [size]="17" />{{ busy() ? 'Signing in...' : 'Sign in' }}</button>
        </form>
        <footer><i></i> Redacted evidence only</footer>
      </section>
    </main>
  `,
  styles: [`
    .login-view{min-height:100vh;display:grid;grid-template-columns:minmax(300px,1fr) minmax(380px,520px);background:#e9edeb}.identity{display:flex;align-items:flex-end;gap:18px;padding:clamp(32px,6vw,76px);color:#f2f5f3;background:#243238;border-bottom:8px solid #d7a728}.emblem{width:62px;height:62px;display:grid;place-items:center;flex:0 0 auto;color:#243238;background:#e1b438;border-radius:4px}.identity p,.identity span{margin:0;color:#b7c1c2;font-size:10px;text-transform:uppercase}.identity h1{margin:7px 0;font-size:48px;line-height:1}.login-panel{align-self:center;margin:34px;padding:30px;background:#fbfcfa;border:1px solid #b9c4c1;border-radius:4px}.login-panel header{display:grid;gap:5px;margin-bottom:20px}.login-panel header span{color:#687579;font-size:10px;text-transform:uppercase}.login-panel header strong{font-size:22px}.role-strip{display:grid;grid-template-columns:repeat(4,1fr);margin-bottom:19px;border:1px solid #bbc5c2;border-radius:3px;overflow:hidden}.role-strip button{min-width:0;padding:8px 3px;color:#536166;background:#e9edeb;border:0;border-right:1px solid #bbc5c2;border-bottom:1px solid #bbc5c2;font-size:9px;text-transform:uppercase;cursor:pointer}.role-strip button.active{color:#202d32;background:#e2b537;font-weight:750}.login-panel form{display:grid}.login-panel form button{height:44px;display:flex;gap:8px}.error{display:flex;align-items:flex-start;gap:7px;margin:0 0 14px;padding:9px;color:#8d302a;background:#f9e9e6;font-size:11px}.login-panel footer{display:flex;align-items:center;justify-content:center;gap:7px;margin-top:21px;color:#687579;font-size:9px;text-transform:uppercase}.login-panel footer i{width:7px;height:7px;background:#347554;border-radius:50%}
    @media(max-width:820px){.login-view{grid-template-columns:1fr;align-content:start}.identity{align-items:center;padding:24px}.identity h1{font-size:32px}.login-panel{width:min(520px,calc(100% - 24px));margin:28px auto;padding:22px}.role-strip{grid-template-columns:repeat(2,1fr)}}
  `],
})
export class LoginPage {
  private readonly fb = inject(FormBuilder);
  private readonly router = inject(Router);
  readonly auth = useAuth();
  readonly busy = signal(false);
  readonly error = signal('');
  readonly profiles = [
    { label: 'Manager', username: 'manager', password: 'Data#536' },
    { label: 'Annotator A', username: 'annotator_a', password: 'Annotate#536' },
    { label: 'Annotator B', username: 'annotator_b', password: 'Compare#536' },
    { label: 'Adjudicator', username: 'adjudicator', password: 'Decide#536' },
    { label: 'Reviewer', username: 'reviewer', password: 'Review#536' },
    { label: 'Auditor', username: 'auditor', password: 'Audit#536' },
    { label: 'Admin', username: 'admin', password: 'Admin#536' },
  ];
  readonly form = this.fb.nonNullable.group({ username: ['manager', Validators.required], password: ['Data#536', Validators.required] });

  select(username: string, password: string): void { this.form.setValue({ username, password }); }
  submit(): void {
    if (this.form.invalid) return;
    this.busy.set(true);
    this.error.set('');
    const { username, password } = this.form.getRawValue();
    this.auth.login(username, password).subscribe({
      next: () => { this.busy.set(false); void this.router.navigate(['/datasets']); },
      error: (error) => { this.busy.set(false); this.error.set(apiErrorMessage(error)); },
    });
  }
}
