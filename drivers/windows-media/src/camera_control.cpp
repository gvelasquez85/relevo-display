// Camera lifecycle utility, NOT the required IMFMediaSource COM provider.
#include <windows.h>
#include <mfapi.h>
#include <mfvirtualcamera.h>
#include <wrl/client.h>
#include <iostream>
#include <string>
using Microsoft::WRL::ComPtr;
// Reserved project identity; a provider implementing it must be installed first.
static constexpr wchar_t SourceId[]=L"{C869639B-87A0-49DC-951B-E53D34F059BA}";
static HRESULT run(const std::wstring& action) {
 HKEY key=nullptr;
 std::wstring path=std::wstring(L"CLSID\\")+SourceId+L"\\InprocServer32";
 if(RegOpenKeyExW(HKEY_CLASSES_ROOT,path.c_str(),0,KEY_READ,&key)!=ERROR_SUCCESS) {
  std::wcerr<<L"Relevo camera COM provider is not installed. No camera created.\n";
  return REGDB_E_CLASSNOTREG;
 }
 RegCloseKey(key);
 ComPtr<IMFVirtualCamera> camera;
 HRESULT hr=MFCreateVirtualCamera(MFVirtualCameraType_SoftwareCameraSource,
  MFVirtualCameraLifetime_System,MFVirtualCameraAccess_CurrentUser,
  L"Relevo Camera",SourceId,nullptr,0,&camera);
 if(FAILED(hr))return hr;
 if(action==L"start")hr=camera->Start(nullptr);
 else hr=camera->Remove();
 HRESULT shutdown=camera->Shutdown();
 return FAILED(hr)?hr:shutdown;
}
int wmain(int argc,wchar_t** argv) {
 if(argc!=2 || (std::wstring(argv[1])!=L"start" && std::wstring(argv[1])!=L"remove")) {
  std::wcerr<<L"Usage: RelevoCameraControl start|remove\n";return 2;
 }
 HRESULT hr=CoInitializeEx(nullptr,COINIT_MULTITHREADED);
 if(FAILED(hr))return 1;
 hr=MFStartup(MF_VERSION,MFSTARTUP_FULL);
 if(SUCCEEDED(hr)){hr=run(argv[1]);MFShutdown();}
 CoUninitialize();
 if(FAILED(hr)){std::wcerr<<L"Camera operation failed: 0x"<<std::hex<<static_cast<unsigned long>(hr)<<L"\n";return 1;}
 return 0;
}
