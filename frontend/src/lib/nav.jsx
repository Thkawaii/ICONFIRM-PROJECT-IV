import { createContext, useCallback, useContext, useState } from 'react';
const NavContext = createContext(null);
export function NavProvider({
  initialView,
  children
}) {
  const [view, setView] = useState(initialView);
  const [params, setParams] = useState({});
  const navigate = useCallback((nextView, nextParams = {}) => {
    setView(nextView);
    setParams(nextParams);
  }, []);
  return <NavContext.Provider value={{
    view,
    params,
    navigate
  }}>
      {children}
    </NavContext.Provider>;
}
function useNavContext() {
  const ctx = useContext(NavContext);
  if (!ctx) throw new Error('useNav hooks ต้องถูกเรียกภายใน <NavProvider> เท่านั้น');
  return ctx;
}
export function useAppNavigate() {
  return useNavContext().navigate;
}
export function useAppParams() {
  return useNavContext().params;
}
export function useAppView() {
  return useNavContext().view;
}
