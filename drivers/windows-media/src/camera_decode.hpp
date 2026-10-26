#pragma once
#ifdef _WIN32
#include <windows.h>
#include <wincodec.h>
#include <wrl/client.h>
#include <cstdint>
#include <vector>
namespace relevo {
// Runs on an MTA initialized by caller. Validate the encoded dimensions before allocation.
// Output fixed 32-bit BGRX, top-down; the future MF source may convert to NV12 explicitly.
inline HRESULT decode_camera_jpeg(const std::vector<uint8_t>& jpeg,
 std::vector<uint8_t>& bgrx, UINT& width, UINT& height) {
 bgrx.clear();width=height=0;
 if(jpeg.size()<4 || jpeg.size()>512*1024)return E_INVALIDARG;
 using Microsoft::WRL::ComPtr;
 ComPtr<IWICImagingFactory> factory;ComPtr<IWICStream> stream;
 ComPtr<IWICBitmapDecoder> decoder;ComPtr<IWICBitmapFrameDecode> frame;
 ComPtr<IWICFormatConverter> convert;
 HRESULT hr=CoCreateInstance(CLSID_WICImagingFactory,nullptr,CLSCTX_INPROC_SERVER,IID_PPV_ARGS(&factory));
 if(FAILED(hr))return hr;
 if(FAILED(hr=factory->CreateStream(&stream)))return hr;
 if(FAILED(hr=stream->InitializeFromMemory(const_cast<BYTE*>(jpeg.data()),static_cast<DWORD>(jpeg.size()))))return hr;
 if(FAILED(hr=factory->CreateDecoderFromStream(stream.Get(),nullptr,WICDecodeMetadataCacheOnDemand,&decoder)))return hr;
 GUID container{};if(FAILED(hr=decoder->GetContainerFormat(&container)))return hr;
 if(container!=GUID_ContainerFormatJpeg)return WINCODEC_ERR_BADIMAGE;
 UINT count=0;if(FAILED(hr=decoder->GetFrameCount(&count)))return hr;
 if(count!=1)return WINCODEC_ERR_BADIMAGE;
 if(FAILED(hr=decoder->GetFrame(0,&frame)))return hr;
 UINT w=0,h=0;if(FAILED(hr=frame->GetSize(&w,&h)))return hr;
 if(w==0 || h==0 || w>640 || h>480)return WINCODEC_ERR_IMAGESIZEOUTOFRANGE;
 if(FAILED(hr=factory->CreateFormatConverter(&convert)))return hr;
 if(FAILED(hr=convert->Initialize(frame.Get(),GUID_WICPixelFormat32bppBGR,
   WICBitmapDitherTypeNone,nullptr,0,WICBitmapPaletteTypeCustom)))return hr;
 std::vector<uint8_t> output(static_cast<size_t>(w)*h*4);
 if(FAILED(hr=convert->CopyPixels(nullptr,w*4,static_cast<UINT>(output.size()),output.data())))return hr;
 bgrx.swap(output);width=w;height=h;return S_OK;
}
}
#endif
