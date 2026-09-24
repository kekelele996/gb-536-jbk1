import { Routes } from '@angular/router';
import { authGuard, guestGuard, roleGuard } from './guards';

export const routes: Routes = [
  { path: 'login', canActivate: [guestGuard], loadComponent: () => import('../pages/login.page').then((module) => module.LoginPage) },
  { path: 'datasets', canActivate: [authGuard], loadComponent: () => import('../pages/datasets.page').then((module) => module.DatasetsPage) },
  { path: 'schemas', canActivate: [authGuard], loadComponent: () => import('../pages/schemas.page').then((module) => module.SchemasPage) },
  { path: 'annotations', canActivate: [authGuard], loadComponent: () => import('../pages/annotations.page').then((module) => module.AnnotationsPage) },
  { path: 'adjudication', canActivate: [authGuard], loadComponent: () => import('../pages/adjudication.page').then((module) => module.AdjudicationPage) },
  { path: 'audit', canActivate: [authGuard, roleGuard('auditor', 'adjudicator', 'admin')], loadComponent: () => import('../pages/audit.page').then((module) => module.AuditPage) },
  { path: '', pathMatch: 'full', redirectTo: 'datasets' },
  { path: '**', redirectTo: 'datasets' },
];
