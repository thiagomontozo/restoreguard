export type User={UserID:string;OrganizationID:string;Email:string;DisplayName:string;Roles:string[]};
export type Asset={id:string;name:string;type:string;environment:string;criticality:'LOW'|'MEDIUM'|'HIGH'|'CRITICAL';ownerName:string;team:string;description:string;enabled:boolean;createdAt:string};
export type Policy={id:string;name:string;protectedAssetId:string;rpoTargetSeconds:number;rtoTargetSeconds:number;schedule:string;enabled:boolean};
export type Drill={id:string;protectedAssetId:string;status:string;recoveryStatus?:string;confidence?:string;measuredRpoSeconds?:number;measuredRtoSeconds?:number;rpoResult?:string;rtoResult?:string;summary:string;createdAt:string};
export type Overview={protectedAssets:number;verifiedAssets:number;neverTested:number;rpoFailures:number;rtoFailures:number;failedDrills:number;inconclusiveDrills:number;runningDrills:number;recoveryCoveragePercent:number};
