import {useQuery} from '@tanstack/react-query';
import {AlertTriangle,CheckCircle2,Clock3,Database,FlaskConical,XCircle} from 'lucide-react';
import {api} from '../api';
import {Page} from '../components/Page';
import type {Overview as OverviewType} from '../types';

const cards=[['Protected Assets','protectedAssets',Database],['Verified Recovery','verifiedAssets',CheckCircle2],['Never Tested','neverTested',FlaskConical],['RPO Failures','rpoFailures',AlertTriangle],['RTO Failures','rtoFailures',Clock3],['Failed Drills','failedDrills',XCircle]] as const;

export function Overview(){
	const {data,isLoading,error}=useQuery({queryKey:['overview'],queryFn:()=>api<{data:OverviewType;coverageExplanation:string}>('/api/v1/overview')});
	const metrics=data?.data;
	return <Page title="Recovery Overview" description="Recovery readiness across protected assets, measured from controlled drills and persisted evidence.">
		<div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">{cards.map(([label,key,Icon])=><article className="card" key={key}><div className="flex items-start justify-between"><div><p className="text-sm font-medium text-slate-600">{label}</p><p className="mt-2 text-3xl font-bold">{isLoading?'—':metrics?.[key]??0}</p></div><span className="rounded-lg bg-slate-100 p-2 text-navy"><Icon aria-hidden size={20}/></span></div></article>)}</div>
		{error&&<p role="alert" className="mt-4 text-danger">Overview could not be loaded.</p>}
		<section className="card mt-6"><div className="flex items-end justify-between gap-4"><div><p className="eyebrow">Recovery coverage</p><p className="mt-2 text-4xl font-bold">{metrics?.recoveryCoveragePercent??0}%</p></div><p className="max-w-2xl text-sm text-slate-600">{data?.coverageExplanation??'Coverage is calculated from assets with a verified drill. It is not a guarantee of disaster recovery.'}</p></div><div className="mt-5 h-2 overflow-hidden rounded-full bg-slate-100"><div aria-label={`${metrics?.recoveryCoveragePercent??0}% recovery coverage`} className="h-full rounded-full bg-blue" style={{width:`${metrics?.recoveryCoveragePercent??0}%`}}/></div></section>
		<section className="mt-6 grid gap-4 lg:grid-cols-3">{[['Backup Availability','Is a usable snapshot available?'],['Restore Success','Did the isolated restore complete?'],['Validation Success','Did required checks pass?'],['RPO Compliance','Was snapshot age within target?'],['RTO Compliance','Was recovery ready within target?'],['Evidence Freshness','Is recent integrity metadata available?']].map(([title,body])=><div className="card" key={title}><p className="font-bold">{title}</p><p className="mt-1 text-sm text-slate-600">{body}</p></div>)}</section>
	</Page>
}
