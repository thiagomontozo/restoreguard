import {createContext,useContext} from 'react';
import type {User} from './types';
export type AuthContextValue={user:User|null;loading:boolean;login:(email:string,password:string)=>Promise<void>;logout:()=>Promise<void>;can:(permission:string)=>boolean};
export const AuthContext=createContext<AuthContextValue|null>(null);
export function useAuth(){const value=useContext(AuthContext);if(!value)throw new Error('AuthProvider is missing');return value}
