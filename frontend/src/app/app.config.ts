import { ApplicationConfig, importProvidersFrom } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import {
  Archive, Brackets, Check, CircleCheck, CircleDot, CircleOff, ClipboardList,
  Database, FileJson2, Filter, Gavel, GitCompareArrows, Layers3, LockKeyhole,
  LogIn, LogOut, LucideAngularModule, PanelRightOpen, Plus, RefreshCw, Save,
  ScanText, Search, ShieldCheck, Tag, Tags, TriangleAlert, UserCheck, X,
} from 'lucide-angular';
import { routes } from '../router/app.routes';
import { authInterceptor } from '../api/auth.interceptor';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes),
    provideHttpClient(withInterceptors([authInterceptor])),
    provideAnimationsAsync(),
    importProvidersFrom(LucideAngularModule.pick({
      Archive, Brackets, Check, CircleCheck, CircleDot, CircleOff, ClipboardList,
      Database, FileJson2, Filter, Gavel, GitCompareArrows, Layers3, LockKeyhole,
      LogIn, LogOut, PanelRightOpen, Plus, RefreshCw, Save, ScanText, Search,
      ShieldCheck, Tag, Tags, TriangleAlert, UserCheck, X,
    })),
  ],
};
